package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"agent/algorand/pkg/watch"
	"agent/api/v1/model"
	watch2 "agent/pkg/watch"
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

var (
	// metrikaPlatformAddr = "http://d42e-195-167-104-122.eu.ngrok.io"
	metrikaPlatformAddr = "http://localhost:8000"
	// metrikaPlatformURI  = "/api/v1/algorand/testnet"
	metrikaPlatformURI = "/"

	// agentMetricsAddr HTTP address for serving prometheus metrics
	agentMetricsAddr = ":9000"
)

func init() {
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(agentMetricsAddr, nil)
	}()
}

type GenericThing struct {
	Timestamp int64 `json:"timestamp"`
}

type SpecificThing struct {
	GenericThing
	OtherField string `json:"other_field"`
}

func main() {
	fmt.Println("Hello, Agent!")

	agentUUID, err := uuid.NewUUID()
	if err != nil {
		logrus.Fatal(err)
	}

	url, err := url.Parse(metrikaPlatformAddr + metrikaPlatformURI)
	if err != nil {
		logrus.Fatal(err)
	}

	conf := publisher.HTTPConf{
		URL:            url.String(),
		UUID:           agentUUID.String(),
		DefaultTimeout: 10 * time.Second,
		MaxBatchLen:    1000,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishFreq:    5 * time.Second,
		MetricTTL:      30 * time.Minute,
	}

	ch := make(chan interface{}, 10000)
	pub := publisher.NewHTTP(ch, conf)

	wg := &sync.WaitGroup{}
	pub.Start(wg)

	w := watch.NewAlgodRestartWatch(watch.AlgodRestartWatchConf{
		Path: "/var/lib/algorand/algod.pid",
	}, nil)

	rand.Seed(time.Now().UnixNano())
	go func() {
		for {
			<-time.After(time.Duration(rand.Intn(5)) * time.Millisecond)
			health := model.NodeHealthMetric{
				Metric: model.NewMetric(true),
				State:  model.NodeStateUp,
			}
			body, _ := json.Marshal(health)

			m := model.MetricPlatform{
				Type:      "foobar-type",
				Timestamp: time.Now().UnixNano(),
				NodeState: model.NodeStateUp,
				Body:      body,
			}
			ch <- m

			// logrus.Debug("[test-watch] sending test ", m.Timestamp)
		}
	}()

	w.Subscribe(ch)
	watch2.Start(w)

	forever := make(chan bool)
	<-forever
}
