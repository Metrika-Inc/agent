package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
)

var (
	defaultTimeout      = 10 * time.Second
	maxAccumulatorMsg   = 1000
	maxAccumulatorMsgSz = uint(1024 * 1024) // 1MB
	publishFreq         = 5 * time.Second
)

type Metric struct {
	Type string `json:"type"`
	Body []byte `json:"body"`
}

type MetrikaMessage struct {
	Data []Metric `json:"data"`
	UUID string   `json:"uid" binding:"required,uuid"`
}

type HTTPConf struct {
	// PublishURL is the publisher's endpoint URL
	URL  *url.URL
	UUID string
}

type HTTP struct {
	receiveCh <-chan interface{}

	conf   HTTPConf
	client *http.Client
}

func NewHTTP(ch <-chan interface{}, conf HTTPConf) *HTTP {
	return &HTTP{
		client:    http.DefaultClient,
		conf:      conf,
		receiveCh: ch,
	}
}

func (h *HTTP) newRequestJSON(ctx context.Context, msg MetrikaMessage) (*http.Request, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.conf.URL.String(), strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (h *HTTP) publish(ctx context.Context, acc accumulator) error {
	reqCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	metrikaMsg := MetrikaMessage{
		Data: acc,
		UUID: h.conf.UUID,
	}

	req, err := h.newRequestJSON(reqCtx, metrikaMsg)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s returned %d", req.URL, resp.StatusCode)
	}

	return nil
}

func (h *HTTP) Start(wg *sync.WaitGroup) {
	ctx := context.Background()
	acc := accumulator{}
	publishTicker := time.NewTicker(publishFreq)
	var lastPublishT time.Time

	logrus.Debug("[http] start publisher")
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case msg, ok := <-h.receiveCh:
				// accumulate the metric and if accumulator is full
				// publish a message to Metrika's platform
				if !ok {
					logrus.Error("[http] receive channel closed")

					return
				}

				metric, ok := msg.(Metric)
				if !ok {
					logrus.Error("[http] type assertion failed")
				}
				acc.Add(metric)

				logrus.Debug("[http] got metric, new acc len: ", len(acc))

				if acc.IsFull() || len(acc) > maxAccumulatorMsg {
					logrus.Info("[http] accumulator full, publishing")
					if err := h.publish(ctx, acc); err != nil {
						logrus.Error("[http] publish error: ", err)
					}
					lastPublishT = time.Now()
					acc.Clear()
				}
			case <-publishTicker.C:
				// ensure we publish any accumulated metric
				// within a reasonable period
				pubDiffT := time.Now().Sub(lastPublishT)
				if len(acc) > 0 && pubDiffT > publishFreq {
					logrus.Info("[http] periodic publishing")
					if err := h.publish(ctx, acc); err != nil {
						logrus.Error("[http] publish error: ", err)
					}
					acc.Clear()
				}
			}
		}

	}()
}

type accumulator []Metric

func (a *accumulator) Add(msg Metric) {
	*a = append(*a, msg)
}

func (a *accumulator) Clear() {
	*a = (*a)[:0]
}

func (a accumulator) IsFull() bool {
	var sz uint
	for _, m := range a {
		sz += uint(unsafe.Sizeof(m))

		if sz >= maxAccumulatorMsgSz {
			return true
		}
	}

	return false
}
