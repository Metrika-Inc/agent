package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"agent/internal/pkg/factory"
	"agent/internal/pkg/global"
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

func registerWatchers() error {
	watchersEnabled := []watch.Watcher{}

	for _, watcherConf := range global.AgentRuntimeConfig.Runtime.Watchers {
		w := factory.NewWatcherByType(*watcherConf)
		if w == nil {
			logrus.Fatalf("watcher factory returned nil for type: %v", watcherConf.Type)
		}

		watchersEnabled = append(watchersEnabled, w)
	}

	if err := global.WatcherRegistry.Register(watchersEnabled...); err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("Hello, Agent!")

	if err := global.LoadDefaultConfig(); err != nil {
		logrus.Fatal(err)
	}

	agentUUID, err := uuid.NewUUID()
	if err != nil {
		logrus.Fatal(err)
	}

	url, err := url.Parse(global.AgentRuntimeConfig.Platform.Addr +
		global.AgentRuntimeConfig.Platform.URI)
	if err != nil {
		logrus.Fatal(err)
	}

	timesync.Default.Start()
	if timesync.Default.SyncNow(); err != nil {
		logrus.Error("Could not sync with NTP server: ", err)
	}

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

	if err := global.WatcherRegistry.Start(ch); err != nil {
		logrus.Fatal(err)
	}

	forever := make(chan bool)
	<-forever
}
