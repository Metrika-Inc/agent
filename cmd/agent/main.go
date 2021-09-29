package main

import (
	"agent/publisher"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	metrikaPlatformAddr = "http://0.0.0.0:8000"
	metrikaPlatformURI  = "/api/v1/algorand/testnet"
)

func init() {
	// logrus.SetLevel(logrus.DebugLevel)
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

	wg.Add(1)
	go func() {
		defer wg.Done()

		http.HandleFunc("/", metrikaHandler)
		log.Fatal(http.ListenAndServe(":8000", nil))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				msgUUID, err := uuid.NewUUID()
				if err != nil {
					logrus.Fatal(err)
				}
				body := msgUUID.String()
				m := publisher.Metric{
					Type: "foo",
					Body: []byte(fmt.Sprintf("{'key': %s}", body)),
				}

				select {
				case ch <- m:
					logrus.Debugf("sent metric to publisher %v", msgUUID)
				case <-time.After(5 * time.Second):
					logrus.Errorf("timeout sending metric to publisher %v", msgUUID)
				}
			}
		}
	}()

	forever := make(chan bool)
	<-forever
}
