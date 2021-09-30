package main

import (
	"agent/algorand/pkg/watch"
	watch2 "agent/pkg/watch"
	"agent/publisher"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
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
	metrikaPlatformAddr = "http://d42e-195-167-104-122.eu.ngrok.io"
	metrikaPlatformURI  = "/api/v1/algorand/testnet"
)

func init() {
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

}

func metrikaHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Error(err)
	}

	var msg publisher.MetrikaMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logrus.Fatal(err)
	}

	for _, metric := range msg.Data {
		logrus.Info(string(metric.Body))
	}
	fmt.Fprintf(w, "Metrika Platform: %v\n", 200)
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
		URL:  url,
		UUID: agentUUID.String(),
	}

	ch := make(chan interface{}, 10)
	pub := publisher.NewHTTP(ch, conf)

	wg := &sync.WaitGroup{}
	pub.Start(wg)

	//w := watch.NewAlgorandLogWatch(watch.AlgorandLogWatchConf{
	//	Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/node_out.log",
	//}, nil)

	w := watch.NewAlgodRestartWatch(watch.AlgodRestartWatchConf{
		Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/algod.pid",
	}, nil)

	w.Subscribe(ch)
	watch2.Start(w)

	wg.Add(1)
	go func() {
		defer wg.Done()

		http.HandleFunc("/", metrikaHandler)
		log.Fatal(http.ListenAndServe(":8000", nil))
	}()

	//wg.Add(1)
	//go func() {
	//	defer wg.Done()
	//
	//	ticker := time.NewTicker(100 * time.Millisecond)
	//	for {
	//		select {
	//		case <-ticker.C:
	//			msgUUID, err := uuid.NewUUID()
	//			if err != nil {
	//				logrus.Fatal(err)
	//			}
	//			body := msgUUID.String()
	//			m := publisher.Metric{
	//				Type: "foo",
	//				Body: []byte(fmt.Sprintf("{'key': %s}", body)),
	//			}
	//
	//			select {
	//			case ch <- m:
	//				logrus.Debugf("sent metric to publisher %v", msgUUID)
	//			case <-time.After(5 * time.Second):
	//				logrus.Errorf("timeout sending metric to publisher %v", msgUUID)
	//			}
	//		}
	//	}
	//}()

	forever := make(chan bool)
	<-forever
}
