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
	"agent/internal/pkg/discover/utils"
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
	discoverer        *utils.NodeDiscoverer
	blockchain        global.Chain
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

func defaultSystemdWatchers() []watch.Watcher {
	sdwConf := watch.SystemdServiceWatchConf{Discoverer: discoverer}
	sdw, err := watch.NewSystemdServiceWatch(sdwConf)
	if err != nil {
		zap.S().Fatalw("cannot start node systemd watcher without regular expression or discoverer", zap.Error(err))
	}

	svc := discoverer.SystemdService()
	if svc == nil || svc.Name == "" {
		zap.S().Fatal("got nil systemd service object or empty unit name")
	}

	dw := []watch.Watcher{sdw}

	// Log watch for event generation
	logEvs := blockchain.LogEventsList()

	// Docker container watch (logs)
	logWatch, err := watch.NewJournaldLogWatch(watch.JournaldLogWatchConf{
		UnitName: svc.Name,
		Events:   logEvs,
	})
	if err != nil {
		zap.S().Fatalw("cannot build journald log watch, this is probably a configuration error", zap.Error(err))
	}

	// start log watcher independently if conditions for it are met
	go logWatch.PendingStart(subscriptions...)

	return dw
}

func defaultDockerWatchers() []watch.Watcher {
	dw := []watch.Watcher{}

	// Log watch for event generation
	logEvs := blockchain.LogEventsList()

	// Docker container watch
	conf := watch.ContainerWatchConf{Discoverer: discoverer}

	w, err := watch.NewContainerWatch(conf)
	if err != nil {
		zap.S().Fatalw("failed to create watcher", zap.Error(err))
	}
	dw = append(dw, w)

	// Docker container watch (logs)
	logWatch := watch.NewDockerLogWatch(watch.DockerLogWatchConf{
		ContainerName: discoverer.DockerContainer().Names[0],
		Events:        logEvs,
	})
	// start log watcher independently if conditions for it are met
	go logWatch.PendingStart(subscriptions...)

	zap.S().Debugf("watching containers %v", logWatch.ContainerName)

	return dw
}

func registerWatchers(ctx context.Context, cupdStream *global.ConfigUpdateStream) error {
	watchersEnabled := []watch.Watcher{}

	for _, watcherConf := range global.AgentConf.Runtime.Watchers {
		w, err := factory.NewWatcherByType(*watcherConf)
		if err != nil {
			zap.S().Fatalw("watcher factory returned error", "type", watcherConf.Type, zap.Error(err))
		}
		if w == nil {
			zap.S().Fatalw("watcher factory returned nil", "type", watcherConf.Type)
		}

		watchersEnabled = append(watchersEnabled, w)
	}

	if global.AgentConf.Discovery.Deactivated {
		if err := watch.DefaultWatchRegistry.Register(watchersEnabled...); err != nil {
			return err
		}

		return nil
	}

	// from now on configure discovery related watchers
	// PEF Watch
	urlResetCh := make(chan global.ConfigUpdate, 1)
	eps := blockchain.PEFEndpoints()
	for i, ep := range eps {
		httpConf := watch.HTTPWatchConf{
			Interval:    global.AgentConf.Runtime.SamplingInterval,
			URL:         ep.URL,
			URLUpdateCh: urlResetCh,
			URLIndex:    i,
			Timeout:     global.AgentConf.Platform.TransportTimeout,
			Headers:     nil,
		}
		httpWatch := watch.NewHTTPWatch(httpConf)
		if cupdStream != nil {
			err := cupdStream.Subscribe(global.PEFEndpointsKey, urlResetCh)
			if err != nil {
				zap.S().Fatalw("error subscribing to config update stream", zap.Error(err))
			}
		}

		filter := &openmetrics.PEFFilter{ToMatch: ep.Filters}
		pefConf := watch.PEFWatchConf{Filter: filter}
		watchersEnabled = append(watchersEnabled, watch.NewPEFWatch(pefConf, httpWatch))
	}

	if err := watch.DefaultWatchRegistry.Register(watchersEnabled...); err != nil {
		return err
	}
	watchersEnabled = watchersEnabled[:0]

	if global.AgentConf.Discovery.Systemd.Deactivated && global.AgentConf.Discovery.Docker.Deactivated {
		zap.S().Warn("node discovery is deactivated, the agent will start without monitoring a node")
		return nil
	} else if global.AgentConf.Discovery.Systemd.Deactivated {
		zap.S().Info("systemd discovery mode deactivated by discovery.systemd.glob.deactivated")
	} else if global.AgentConf.Discovery.Docker.Deactivated {
		zap.S().Info("docker discovery mode deactivated by discovery.docker.regex.deactivated")
	}

	c := utils.NodeDiscovererConfig{
		UnitGlob:       global.AgentConf.Discovery.Systemd.Glob,
		ContainerRegex: global.AgentConf.Discovery.Docker.Regex,
	}

	var err error
	discoverer, err = utils.NewNodeDiscoverer(c)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			watchersEnabled := []watch.Watcher{}
			startWatchers := []watch.Watcher{}
			scheme := discoverer.DetectScheme(ctx)

			switch scheme {
			case global.NodeDocker:
				container := discoverer.DockerContainer()
				if container == nil {
					zap.S().Fatal("got docker scheme but container is nil")
				}

				reader, err := utils.NewDockerLogsReader(container.Names[0])
				if err != nil {
					zap.S().Warnw("error creating docker log reader", zap.Error(err))
				} else {
					if err := blockchain.ReconfigureByDockerContainer(container, reader); err != nil {
						zap.S().Warnw("node metadata configuration failed for docker", zap.Error(err))
					}
					reader.Close()
				}

				zap.S().Infow("starting docker watchers")
				startWatchers = defaultDockerWatchers()
			case global.NodeSystemd:
				unit := discoverer.SystemdService()
				if unit == nil {
					zap.S().Fatal("got systemd scheme but systemd unit is nil")
				}

				reader, err := utils.NewJournalReader(unit.Name)
				if err != nil {
					zap.S().Warnw("error creating journald log reader", zap.Error(err))
				} else {
					if err := blockchain.ReconfigureBySystemdUnit(unit, reader); err != nil {
						zap.S().Warnw("node metadata configuration failed for systemd", zap.Error(err))
					}
					reader.Close()
				}

				zap.S().Infow("starting systemd watchers")
				startWatchers = defaultSystemdWatchers()
			default:
				zap.S().Warnw("node discovery returned no errors but scheme is unknown, retrying in 2s", "scheme", scheme)
				<-time.After(2 * time.Second)
				continue
			}
			blockchain.SetRunScheme(scheme)

			watchersEnabled = append(watchersEnabled, startWatchers...)
			if len(watchersEnabled) > 0 {
				for _, w := range watchersEnabled {
					if err := watch.DefaultWatchRegistry.RegisterAndStart(w, subscriptions...); err != nil {
						zap.S().Errorw("error registering node discovery watchers", zap.Error(err))
					}
				}
				break
			}
		}
	}()

	if err := watch.DefaultWatchRegistry.Register(watchersEnabled...); err != nil {
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

	if err := global.LoadAgentConfig(&global.AgentConf); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}
	ctx := context.Background()
	timesync.SetDefault(timesync.NewTimeSync(ctx, global.AgentConf.Runtime.NTPServer, 0))
	zapLevelHandler := setupZapLogger()

	chain, err := discover.AutoConfig(&global.AgentConf, reset)
	if err != nil {
		zap.S().Fatalw("configuration error", zap.Error(err))
	}
	global.SetBlockchainNode(chain)

	if configureOnly {
		zap.S().Info("configure only mode on, exiting")

		os.Exit(0)
	}
	blockchain = global.BlockchainNode()

	if err := global.AgentPrepareStartup(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	ctx, cancel = context.WithCancel(context.Background())
	// setup config update stream
	updCh := blockchain.ConfigUpdateCh()
	var cupdStream *global.ConfigUpdateStream
	if updCh != nil {
		cupdStream = global.NewConfigUpdateStream(global.ConfigUpdateStreamConf{UpdatesCh: updCh})
		cupdStream.Run(ctx)
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
		mux := http.NewServeMux()
		httpsrv = mahttp.StartHTTPServer(httpwg, global.AgentConf.Runtime.HTTPAddr, mux)
		if global.AgentConf.Runtime.MetricsEnabled {
			// attempt register NetDev collector for network metrics
			netdev, err := collector.NewNetDevCollector()
			if err != nil {
				zap.S().Errorw("failed to initialize netdev collector for tracking network metrics", zap.Error(err))
			} else {
				prometheus.MustRegister(netdev)
			}
			mux.Handle("/metrics", mahttp.ValidationMiddleware(promHandler))
		}
		mux.Handle("/loglvl", mahttp.ValidationMiddleware(zapLevelHandler))
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

	multiEmitter := emit.NewMultiEmitter(subscriptions)

	global.DefaultExporterRegisterer.Start(ctx, wg)

	// we should be (almost) ready to publish at this point
	// start default and enabled watchers
	if err := registerWatchers(ctx, cupdStream); err != nil {
		log.Fatal(err)
	}
	if discoverer != nil {
		defer discoverer.Close()
	}

	if err := watch.DefaultWatchRegistry.Start(subscriptions...); err != nil {
		log.Fatal(err)
	}

	log.Infof("finished agent setup")
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
	watch.DefaultWatchRegistry.Stop()
	watch.DefaultWatchRegistry.Wait()

	// stop docker client
	utils.DefaultDockerAdapter.Close()

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
