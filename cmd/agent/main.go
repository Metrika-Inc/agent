package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/factory"
	"agent/internal/pkg/global"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/timesync"
	"agent/pkg/watch"
	"agent/publisher"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "net/http/pprof"
)

var (
	reset         = flag.Bool("reset", false, "Remove existing protocol-related configuration. Restarts the discovery process")
	configureOnly = flag.Bool("configure-only", false, "Exit agent after automatic discovery and validation process")

	ch = make(chan interface{}, 10000)
)

func init() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	if err := global.LoadDefaultConfig(); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	setupZapLogger()

	global.NodeProtocol = discover.AutoConfig(*reset)
	if *configureOnly {
		os.Exit(0)
	}

	zap.S().Info("node discovered: ", discover.Hello())

	timesync.Default.Start(ch)
	if err := timesync.Default.SyncNow(); err != nil {
		zap.S().Errorw("could not sync with NTP server", zap.Error(err))

		ctx := map[string]interface{}{"error": err.Error()}
		timesync.EmitEventWithCtx(timesync.Default, ctx, model.AgentClockNoSyncName, model.AgentClockNoSyncDesc)
	} else {
		timesync.EmitEvent(timesync.Default, model.AgentClockSyncName, model.AgentClockSyncDesc)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(global.AgentRuntimeConfig.Runtime.MetricsAddr, nil)
	}()
}

func setupZapLogger() {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(global.AgentRuntimeConfig.Runtime.Log.Level())
	cfg.OutputPaths = global.AgentRuntimeConfig.Runtime.Log.Outputs
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
	logEvs := discover.LogEventsList()

	regex := discover.ContainerRegex()
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
	eps := global.NodeProtocol.PEFEndpoints()
	for _, ep := range eps {
		httpConf := watch.HttpGetWatchConf{
			Interval: global.AgentRuntimeConfig.Runtime.SamplingInterval,
			Url:      ep.URL,
			Timeout:  global.AgentRuntimeConfig.Platform.TransportTimeout,
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

	for _, watcherConf := range global.AgentRuntimeConfig.Runtime.Watchers {
		w := factory.NewWatcherByType(*watcherConf)
		if w == nil {
			zap.S().Fatalf("watcher factory returned nil for type: %v", watcherConf.Type)
		}

		watchersEnabled = append(watchersEnabled, w)
	}

	watchersEnabled = append(watchersEnabled, defaultWatchers()...)

	if err := global.WatcherRegistry.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	log := zap.S()
	defer log.Sync()

	if err := global.FingerprintSetup(); err != nil {
		log.Fatal("fingerprint initialization error: ", err)
	}

	agentUUID, err := uuid.NewUUID()
	if err != nil {
		log.Fatal(err)
	}

	conf := publisher.TransportConf{
		URL:            global.AgentRuntimeConfig.Platform.Addr,
		UUID:           agentUUID.String(),
		Timeout:        global.AgentRuntimeConfig.Platform.TransportTimeout,
		MaxBatchLen:    global.AgentRuntimeConfig.Platform.BatchN,
		MaxBufferBytes: global.AgentRuntimeConfig.Buffer.Size,
		PublishIntv:    global.AgentRuntimeConfig.Platform.MaxPublishInterval,
		BufferTTL:      global.AgentRuntimeConfig.Buffer.TTL,
		RetryCount:     global.AgentRuntimeConfig.Platform.RetryCount,
	}

	pub := publisher.NewTransport(ch, conf)

	wg := &sync.WaitGroup{}
	pub.Start(wg)

	if err := registerWatchers(); err != nil {
		log.Fatal(err)
	}

	if err := global.WatcherRegistry.Start(ch); err != nil {
		log.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}
