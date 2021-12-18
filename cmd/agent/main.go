package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	algorandWatch "agent/algorand/pkg/watch"
	"agent/internal/pkg/global"
	"agent/pkg/collector"
	"agent/pkg/timesync"
	"agent/pkg/watch"
	"agent/publisher"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

/*
Latest block number
  JsonLogWatch(node.log) > `type=RoundConcluded`, send `Round`
Relay Connections
  NetstatWatch > send outgoing connections to 4160
  RestartWatch > reload config file
Latest protocol version
  HttpGetWatch(/v2/status)
Latest software version
  HttpGetWatch(/versions)
Node restarts
  PidWatch(algod.pid)
Error messages
  JsonLogWatch(node.log) > `type=error`, send all
VoteBroadcast (https://developer.algorand.org/docs/run-a-node/participate/online/#check-that-the-node-is-participating)
  JsonLogWatch(node.log) > `type=VoteBroadcast` > incr state
  TimedWatch > send count, incr state
Sync
  HttpGetWatch(/v2/status) > `sync_time != 0` > sample quickly, send `sync_start`
  HttpGetWatch(/v2/status) > `sync_time == 0` > sample slowly, send `sync_end`
*/

func init() {
	rand.Seed(time.Now().UnixNano())

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(global.AgentRuntimeConfig.Runtime.MetricsAddr, nil)
	}()
}

type GenericThing struct {
	Timestamp int64 `json:"timestamp"`
}

type SpecificThing struct {
	GenericThing
	OtherField string `json:"other_field"`
}

func collectorsFactory(t global.WatchType) collector.Collector {
	var clr collector.Collector
	var err error
	switch t {
	case global.PrometheusConntrack:
		clr, err = collector.NewConntrackCollector()
	case global.PrometheusCPU:
		clr, err = collector.NewCPUCollector()
	case global.PrometheusDiskStats:
		clr, err = collector.NewDiskstatsCollector()
	case global.PrometheusEntropy:
		clr, err = collector.NewEntropyCollector()
	case global.PrometheusFileFD:
		clr, err = collector.NewFileFDStatCollector()
	case global.PrometheusFilesystem:
		clr, err = collector.NewFilesystemCollector()
	case global.PrometheusLoadAvg:
		clr, err = collector.NewLoadavgCollector()
	case global.PrometheusMemInfo:
		clr, err = collector.NewMeminfoCollector()
	case global.PrometheusNetClass:
		clr, err = collector.NewNetClassCollector()
	case global.PrometheusNetDev:
		clr, err = collector.NewNetDevCollector()
	case global.PrometheusSockStat:
		clr, err = collector.NewSockStatCollector()
	case global.PrometheusTextfile:
		clr, err = collector.NewTextFileCollector()
	case global.PrometheusTime:
		clr, err = collector.NewTimeCollector()
	case global.PrometheusUname:
		clr, err = collector.NewUnameCollector()
	case global.PrometheusVMStat:
		clr, err = collector.NewvmStatCollector()
	case global.PrometheusNetNetstat:
		clr, err = collector.NewNetStatCollector()
	case global.PrometheusNetARP:
		clr, err = collector.NewARPCollector()
	case global.PrometheusStat:
		clr, err = collector.NewStatCollector()
	default:
		logrus.Fatal(fmt.Errorf("collector for type %q not found", t))
	}

	if err != nil {
		logrus.Fatal(err)
	}

	return clr
}

func watchersFactory(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	var clr collector.Collector
	var err error
	switch conf.Type {
	case global.AlgorandNodeRestart: // algorand
		w = algorandWatch.NewAlgodRestartWatch(algorandWatch.AlgodRestartWatchConf{
			Path: "/var/lib/algorand/algod.pid",
		}, nil)
	case global.PrometheusConntrack:
		fallthrough
	case global.PrometheusCPU:
		fallthrough
	case global.PrometheusDiskStats:
		fallthrough
	case global.PrometheusEntropy:
		fallthrough
	case global.PrometheusFileFD:
		fallthrough
	case global.PrometheusFilesystem:
		fallthrough
	case global.PrometheusLoadAvg:
		fallthrough
	case global.PrometheusMemInfo:
		fallthrough
	case global.PrometheusNetClass:
		fallthrough
	case global.PrometheusNetDev:
		fallthrough
	case global.PrometheusSockStat:
		fallthrough
	case global.PrometheusTextfile:
		fallthrough
	case global.PrometheusTime:
		fallthrough
	case global.PrometheusUname:
		fallthrough
	case global.PrometheusVMStat:
		fallthrough
	case global.PrometheusNetNetstat:
		fallthrough
	case global.PrometheusNetARP:
		fallthrough
	case global.PrometheusStat:
		clr = collectorsFactory(conf.Type)
		w = collector.NewCollectorWatch(collector.CollectorWatchConf{
			Type:      global.CollectorType(conf.Type),
			Collector: clr,
			Interval:  conf.SamplingInterval,
		})
	default:
		logrus.Fatal(fmt.Errorf("collector for type %q not found", conf.Type))
	}

	if err != nil {
		logrus.Fatal(err)
	}

	return w
}

func registerWatchers() error {
	watchersEnabled := []watch.Watcher{}

	for _, watcherConf := range global.AgentRuntimeConfig.Runtime.Watchers {
		w := watchersFactory(*watcherConf)
		watchersEnabled = append(watchersEnabled, w)
	}

	if err := global.WatcherRegistrar.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("Hello, Agent!")

	agentUUID, err := uuid.NewUUID()
	if err != nil {
		logrus.Fatal(err)
	}

	url, err := url.Parse(global.AgentRuntimeConfig.Platform.Addr +
		global.AgentRuntimeConfig.Platform.URI)
	if err != nil {
		logrus.Fatal(err)
	}

	if timesync.Default.SyncNow(); err != nil {
		logrus.Error("Could not sync with NTP server: ", err)
	}
	timesync.Default.Start()

	conf := publisher.HTTPConf{
		URL:            url.String(),
		UUID:           agentUUID.String(),
		Timeout:        global.AgentRuntimeConfig.Platform.HTTPTimeout,
		MaxBatchLen:    global.AgentRuntimeConfig.Platform.BatchN,
		MaxBufferBytes: global.AgentRuntimeConfig.Buffer.Size,
		PublishIntv:    global.AgentRuntimeConfig.Platform.MaxPublishInterval,
		BufferTTL:      global.AgentRuntimeConfig.Buffer.TTL,
	}

	ch := make(chan interface{}, 10000)
	pub := publisher.NewHTTP(ch, conf)

	wg := &sync.WaitGroup{}
	pub.Start(wg)

	if err := registerWatchers(); err != nil {
		logrus.Fatal(err)
	}

	if err := global.WatcherRegistrar.Start(ch); err != nil {
		logrus.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}
