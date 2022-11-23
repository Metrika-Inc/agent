// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/contrib"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/emit"
	"agent/internal/pkg/global"
	"agent/internal/pkg/mahttp"
	"agent/internal/pkg/publisher"
	"agent/internal/pkg/watch"
	"agent/internal/pkg/watch/factory"
	"agent/pkg/collector"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	reset         bool
	configureOnly bool
	showVersion   bool
	flags         = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	ch            = newSubscriptionChan()
	subscriptions = []chan<- interface{}{}

	wg = &sync.WaitGroup{}

	ctx, pubCtx       context.Context
	cancel, pubCancel context.CancelFunc
	promHandler       = promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true})
)

func newSubscriptionChan() chan interface{} {
	return make(chan interface{}, 1000)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func parseFlags(args []string) error {
	flags.BoolVar(&reset, "reset", false, "Remove existing protocol-related configuration. Restarts the discovery process.")
	flags.BoolVar(&configureOnly, "configure-only", false, "Exit agent after automatic discovery and validation process.")
	flags.BoolVar(&showVersion, "version", false, "Show the metrika agent version and exit.")
	collector.DefineFsPathFlags(flags)

	if err := flags.Parse(args); err != nil {
		return err
	}

	return nil
}

func setupZapLogger() zap.AtomicLevel {
	zapLevelHandler := zap.NewAtomicLevelAt(global.AgentConf.Runtime.Log.Level())
	cfg := zap.NewProductionConfig()
	cfg.Level = zapLevelHandler
	cfg.OutputPaths = global.AgentConf.Runtime.Log.Outputs
	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	opts := []zap.Option{
		zap.AddStacktrace(zapcore.FatalLevel),
		zap.WithClock(timesync.Default),
	}
	l, err := cfg.Build(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to setup zap logging: %v", err))
	}

	// set newly configured logger as default (access via zap.L() // zap.S())
	zap.ReplaceGlobals(l)

	return zapLevelHandler
}

func defaultWatchers() []watch.Watcher {
	dw := []watch.Watcher{}

	// Log watch for event generation
	logEvs := global.BlockchainNode.LogEventsList()

	regex := global.BlockchainNode.ContainerRegex()
	if len(regex) > 0 {
		// Docker container watch
		dw = append(dw, watch.NewContainerWatch(watch.ContainerWatchConf{
			Regex: regex,
		}))

		// Docker container watch (logs)
		dw = append(dw, watch.NewDockerLogWatch(watch.DockerLogWatchConf{
			Regex:  regex,
			Events: logEvs,
		}))

		zap.S().Debugf("watching containers %v", regex)
	}

	// PEF Watch
	eps := global.BlockchainNode.PEFEndpoints()
	for _, ep := range eps {
		httpConf := watch.HTTPWatchConf{
			Interval: global.AgentConf.Runtime.SamplingInterval,
			URL:      ep.URL,
			Timeout:  global.AgentConf.Platform.TransportTimeout,
			Headers:  nil,
		}
		httpWatch := watch.NewHTTPWatch(httpConf)
		filter := &openmetrics.PEFFilter{ToMatch: ep.Filters}
		pefConf := watch.PEFWatchConf{Filter: filter}
		dw = append(dw, watch.NewPEFWatch(pefConf, httpWatch))
	}

	return dw
}

func registerWatchers() error {
	watchersEnabled := []watch.Watcher{}

	for _, watcherConf := range global.AgentConf.Runtime.Watchers {
		w := factory.NewWatcherByType(*watcherConf)
		if w == nil {
			zap.S().Fatalf("watcher factory returned nil for type: %v", watcherConf.Type)
		}

		watchersEnabled = append(watchersEnabled, w)
	}

	watchersEnabled = append(watchersEnabled, defaultWatchers()...)

	if err := factory.DefaultWatchRegistry.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := parseFlags(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(2)
		}
		os.Exit(1)
	}

	if showVersion {
		fmt.Print(global.Version)
		os.Exit(0)
	}

	if err := global.LoadAgentConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}
	zapLevelHandler := setupZapLogger()
	zap.S().Debugw("loaded agent config", "config", global.AgentConf)

	if err := global.AgentPrepareStartup(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	global.BlockchainNode = discover.AutoConfig(reset)
	if configureOnly {
		zap.S().Info("configure only mode on, exiting")

		os.Exit(0)
	}

	timesync.Default.Start(ch)
	if err := timesync.Default.SyncNow(); err != nil {
		zap.S().Errorw("could not sync with NTP server", zap.Error(err))

		ctx := map[string]interface{}{model.ErrorKey: err.Error()}
		timesync.EmitEventWithCtx(timesync.Default, ctx, model.AgentClockNoSyncName)
	} else {
		timesync.EmitEvent(timesync.Default, model.AgentClockSyncName)
	}

	httpwg := &sync.WaitGroup{}

	var httpsrv *http.Server
	if global.AgentConf.Runtime.HTTPAddr != "" {
		httpwg.Add(1)
		httpsrv = mahttp.StartHTTPServer(httpwg, global.AgentConf.Runtime.HTTPAddr)
		if global.AgentConf.Runtime.MetricsEnabled {
			http.Handle("/metrics", mahttp.ValidationMiddleware(promHandler))
		}
		http.Handle("/loglvl", mahttp.ValidationMiddleware(zapLevelHandler))
	}

	log := zap.S()
	defer log.Sync()

	if global.AgentConf.Platform.IsEnabled() {
		pub, err := publisher.NewPlatformPublisher(global.AgentHostname, global.AgentConf.Platform, global.AgentConf.Buffer)
		if err != nil {
			log.Fatalw("failed to initialize metrika platform exporter", zap.Error(err))
		}
		pubCtx, pubCancel = context.WithCancel(context.Background())
		pub.Start(pubCtx, wg)
		subCh := newSubscriptionChan()
		subscriptions = append(subscriptions, subCh)
		global.DefaultExporterRegisterer.Register(pub, subCh)
	}

	if len(global.AgentConf.Runtime.Exporters) > 0 {
		exporters := contrib.SetupEnabledExporters(global.AgentConf.Runtime.Exporters)
		for i := range exporters {
			subCh := newSubscriptionChan()
			subscriptions = append(subscriptions, subCh)
			if err := global.DefaultExporterRegisterer.Register(exporters[i], subCh); err != nil {
				log.Errorw("failed to register an exporter", zap.Error(err))
				continue
			}
		}
	}

	ctx, cancel = context.WithCancel(context.Background())

	multiEmitter := emit.NewMultiEmitter(subscriptions)

	global.DefaultExporterRegisterer.Start(ctx, wg)

	// we should be (almost) ready to publish at this point
	// start default and enabled watchers
	if err := registerWatchers(); err != nil {
		log.Fatal(err)
	}

	if err := factory.DefaultWatchRegistry.Start(subscriptions...); err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	sig := <-sigs
	log.Infof("received OS signal %v", sig)
	log.Debug("agent is shutting down...")

	go func() {
		// force exit if we get more signals after the first one
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

		sig := <-sigs
		log.Warnf("received OS signal %v (forced)", sig)

		os.Exit(1)
	}()

	ev, err := model.New(model.AgentDownName, timesync.Now())
	if err != nil {
		log.Error("error creating event: ", err)
	}

	if err := emit.Ev(multiEmitter, ev); err != nil {
		log.Error("error emitting event: ", err)
	}

	if httpsrv != nil {
		httpctx, httpcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer httpcancel()
		if err := httpsrv.Shutdown(httpctx); err != nil {
			log.Errorw("error shutting down HTTP server", zap.Error(err))
		}
	}

	// wait for goroutine started in startHttpServer() to stop
	httpwg.Wait()

	// stop watchers and wait for goroutine cleanup
	factory.DefaultWatchRegistry.Stop()
	factory.DefaultWatchRegistry.Wait()

	// stop platform publisher if running
	if pubCancel != nil {
		pubCancel()
	}
	// stop other exporters &&
	// wait for buffers to drain
	cancel()
	wg.Wait()

	log.Info("shutdown complete, goodbye")
}
