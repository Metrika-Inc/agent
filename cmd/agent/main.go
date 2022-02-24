package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"agent/internal/pkg/discover"
	"agent/internal/pkg/factory"
	"agent/internal/pkg/global"
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
)

func init() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	if err := global.LoadDefaultConfig(); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	setupZapLogger()

	discover.AutoConfig(*reset)
	if *configureOnly {
		os.Exit(0)
	}

	timesync.Default.Start()
	if err := timesync.Default.SyncNow(); err != nil {
		zap.S().Errorw("could not sync with NTP server", zap.Error(err))
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

func registerWatchers() error {
	watchersEnabled := []watch.Watcher{}

	for _, watcherConf := range global.AgentRuntimeConfig.Runtime.Watchers {
		w := factory.NewWatcherByType(*watcherConf)
		if w == nil {
			zap.S().Fatalf("watcher factory returned nil for type: %v", watcherConf.Type)
		}

		watchersEnabled = append(watchersEnabled, w)
	}

	if err := global.WatcherRegistry.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	log := zap.S()
	defer log.Sync()

	os.Exit(0)
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

	ch := make(chan interface{}, 10000)
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
