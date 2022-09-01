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
	"agent/internal/pkg/buf"
	"agent/internal/pkg/contrib"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/emit"
	"agent/internal/pkg/global"
	"agent/internal/pkg/publisher"
	"agent/internal/pkg/transport"
	"agent/internal/pkg/watch"
	"agent/internal/pkg/watch/factory"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "net/http/pprof"
)

var (
	reset         = flag.Bool("reset", false, "Remove existing protocol-related configuration. Restarts the discovery process")
	configureOnly = flag.Bool("configure-only", false, "Exit agent after automatic discovery and validation process")

	ch            = newSubscriptionChan()
	subscriptions = []chan<- interface{}{ch}
	simpleEmitter = emit.NewSimpleEmitter(ch)

	wg     = &sync.WaitGroup{}
	ctx    context.Context
	cancel context.CancelFunc
)

func newSubscriptionChan() chan interface{} {
	return make(chan interface{}, 1000)
}

func init() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()
}

func setupZapLogger() {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(global.AgentConf.Runtime.Log.Level())
	cfg.OutputPaths = global.AgentConf.Runtime.Log.Outputs
	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}
	cfg.EncoderConfig.EncodeTime = logTimestampMSEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	opts := []zap.Option{
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.WithClock(timesync.Default),
	}
	http.Handle("/loglvl", cfg.Level)
	l, err := cfg.Build(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to setup zap logging: %v", err))
	}

	// set newly configured logger as default (access via zap.L() // zap.S())
	zap.ReplaceGlobals(l)
}

// logTimestampMSEncoder encodes the log timestamp as an int64 from Time.UnixMilli()
func logTimestampMSEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendInt64(t.UnixMilli())
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
		httpConf := watch.HttpGetWatchConf{
			Interval: global.AgentConf.Runtime.SamplingInterval,
			Url:      ep.URL,
			Timeout:  global.AgentConf.Platform.TransportTimeout,
			Headers:  nil,
		}
		httpWatch := watch.NewHttpGetWatch(httpConf)
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

	if err := factory.WatcherRegistry.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := global.LoadDefaultConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	setupZapLogger()

	if err := global.AgentPrepareStartup(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)

		os.Exit(1)
	}

	global.BlockchainNode = discover.AutoConfig(*reset)
	if *configureOnly {
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

	go func() {
		promHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true})
		http.Handle("/metrics", promHandler)
		http.ListenAndServe(global.AgentConf.Runtime.MetricsAddr, nil)
	}()

	log := zap.S()
	defer log.Sync()

	transportConf := transport.PlatformGRPCConf{
		UUID:            global.AgentHostname,
		APIKey:          global.AgentConf.Platform.APIKey,
		TransmitTimeout: global.AgentConf.Platform.TransportTimeout,
		URL:             global.AgentConf.Platform.Addr,
	}

	platform, err := transport.NewPlatformGRPC(transportConf)
	if err != nil {
		log.Fatalw("transport initialize error", zap.Error(err))
	}

	// initialize the buffer for temporary in-memory caching of collected data
	// and its controller for maintaining and accessing the buffer.
	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         global.AgentConf.Platform.BatchN,
		BufDrainFreq:        global.AgentConf.Platform.MaxPublishInterval,
		OnBufRemoveCallback: platform.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(global.AgentConf.Buffer.TTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	// attach the buffer controller to the publisher
	pub := publisher.NewPublisher(publisher.PublisherConf{}, bufCtrl)
	pub.Start(wg)

	ctx, cancel = context.WithCancel(context.Background())
	// register Metrika Platform exporter
	// TODO: make possible to disable in configuration
	global.DefaultExporterRegisterer.Register(pub, ch)

	// register other exporters (if enabled)
	if global.AgentConf.Runtime.UseExporters {
		exporters, err := contrib.GetExporters()
		if err != nil {
			log.Fatalw("could not setup the exporters", zap.Error(err))
		}
		for _, exporter := range exporters {
			subCh := newSubscriptionChan()
			subscriptions = append(subscriptions, subCh)
			global.DefaultExporterRegisterer.Register(exporter, subCh)

		}

	}
	global.DefaultExporterRegisterer.Start(ctx, wg)

	// we should be (almost) ready to publish at this point
	// start default and enabled watchers
	if err := registerWatchers(); err != nil {
		log.Fatal(err)
	}

	if err := factory.WatcherRegistry.Start(subscriptions...); err != nil {
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

	if err := emit.Ev(simpleEmitter, ev); err != nil {
		log.Error("error emitting event: ", err)
	}

	// stop watchers and wait for goroutine cleanup
	factory.WatcherRegistry.Stop()
	factory.WatcherRegistry.Wait()

	// stop platform publisher and other exporters &&
	// wait for buffers to drain
	pub.Stop()
	cancel()
	wg.Wait()

	log.Info("shutdown complete, goodbye")
}
