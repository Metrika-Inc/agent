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

func collectorsFactory(t watch.WatchType) collector.Collector {
	var clr collector.Collector
	var err error
	switch t {
	case watch.PrometheusConntrack:
		clr, err = collector.NewConntrackCollector()
	case watch.PrometheusCPU:
		clr, err = collector.NewCPUCollector()
	case watch.PrometheusDiskStats:
		clr, err = collector.NewDiskstatsCollector()
	case watch.PrometheusEntropy:
		clr, err = collector.NewEntropyCollector()
	case watch.PrometheusFileFD:
		clr, err = collector.NewFileFDStatCollector()
	case watch.PrometheusFilesystem:
		clr, err = collector.NewFilesystemCollector()
	case watch.PrometheusLoadAvg:
		clr, err = collector.NewLoadavgCollector()
	case watch.PrometheusMemInfo:
		clr, err = collector.NewMeminfoCollector()
	case watch.PrometheusNetClass:
		clr, err = collector.NewNetClassCollector()
	case watch.PrometheusNetDev:
		clr, err = collector.NewNetDevCollector()
	case watch.PrometheusSockStat:
		clr, err = collector.NewSockStatCollector()
	case watch.PrometheusTextfile:
		clr, err = collector.NewTextFileCollector()
	case watch.PrometheusTime:
		clr, err = collector.NewTimeCollector()
	case watch.PrometheusUname:
		clr, err = collector.NewUnameCollector()
	case watch.PrometheusVMStat:
		clr, err = collector.NewvmStatCollector()
	case watch.PrometheusNetNetstat:
		clr, err = collector.NewNetStatCollector()
	case watch.PrometheusNetARP:
		clr, err = collector.NewARPCollector()
	case watch.PrometheusStat:
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
	case watch.AlgorandNodeRestart: // algorand
		w = algorandWatch.NewAlgodRestartWatch(algorandWatch.AlgodRestartWatchConf{
			Path: "/var/lib/algorand/algod.pid",
		}, nil)
	case watch.PrometheusConntrack:
		fallthrough
	case watch.PrometheusCPU:
		fallthrough
	case watch.PrometheusDiskStats:
		fallthrough
	case watch.PrometheusEntropy:
		fallthrough
	case watch.PrometheusFileFD:
		fallthrough
	case watch.PrometheusFilesystem:
		fallthrough
	case watch.PrometheusLoadAvg:
		fallthrough
	case watch.PrometheusMemInfo:
		fallthrough
	case watch.PrometheusNetClass:
		fallthrough
	case watch.PrometheusNetDev:
		fallthrough
	case watch.PrometheusSockStat:
		fallthrough
	case watch.PrometheusTextfile:
		fallthrough
	case watch.PrometheusTime:
		fallthrough
	case watch.PrometheusUname:
		fallthrough
	case watch.PrometheusVMStat:
		fallthrough
	case watch.PrometheusNetNetstat:
		fallthrough
	case watch.PrometheusNetARP:
		fallthrough
	case watch.PrometheusStat:
		clr = collectorsFactory(conf.Type)
		w = watch.NewCollectorWatch(watch.CollectorWatchConf{
			Type:      watch.WatchType(conf.Type),
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
