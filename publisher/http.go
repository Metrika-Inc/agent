package publisher

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/pkg/timesync"

	"github.com/sirupsen/logrus"
)

type platformState int32

var (
	state *AgentState
)

func init() {
	state = new(AgentState)
	state.Reset()
}

const (
	platformStateUknown platformState = iota
	platformStateUp                   = iota
	platformStateDown                 = iota
)

type HTTPConf struct {
	// URL where the publisher will be pushing metrics to.
	URL string

	// UUID the agent's unique identifier
	UUID string

	// DefaultTimeout default timeout for HTTP requests to the platform
	DefaultTimeout time.Duration

	// MaxBatchLen max number of metrics published at once to the platform
	MaxBatchLen int

	// MaxBufferBytes max size of the buffer
	MaxBufferBytes uint

	// PublishFreq is the (periodic) publishing interval
	PublishFreq time.Duration

	// MetricTTL max duration a metric can stay in the buffer
	MetricTTL time.Duration
}

type HTTP struct {
	receiveCh <-chan interface{}

	conf    HTTPConf
	client  *http.Client
	buffer  buf.Buffer
	closeCh chan interface{}
}

func NewHTTP(ch <-chan interface{}, conf HTTPConf) *HTTP {
	state := new(AgentState)
	state.Reset()

	return &HTTP{
		client:    http.DefaultClient,
		conf:      conf,
		receiveCh: ch,
		buffer:    buf.NewPriorityBuffer(conf.MaxBufferBytes, conf.MetricTTL),
		closeCh:   make(chan interface{}),
	}
}

func publish(ctx context.Context, data model.MetricBatch) (int64, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	uuid, err := stringFromContext(ctx, AgentUUIDContextKey)
	if err != nil {
		return 0, err
	}

	metrikaMsg := model.MetrikaMessage{
		Data: data,
		UUID: uuid,
	}

	req, err := newHTTPRequestFromContext(reqCtx, metrikaMsg)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("POST %s %d", req.URL, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Error("failed to read response body: ", err)
		return 0, nil
	}
	timestamp, err := strconv.ParseInt(string(body), 10, 64)
	if err != nil {
		logrus.Errorf("parseInt on response body failed: %v", err)
	}

	return timestamp, nil
}

func (h *HTTP) NewPublishFuncWithContext(ctx context.Context) func(b buf.ItemBatch) error {
	publishFunc := func(b buf.ItemBatch) error {
		// TODO: consider reusing batch slice
		batch := make(model.MetricBatch, 0, len(b)+1)
		for _, item := range b {
			m, ok := item.Data.(model.MetricPlatform)
			if !ok {
				logrus.Warnf("unrecognised type %T", item.Data)

				// ignore
				continue
			}
			batch = append(batch, m)
		}

		errCh := make(chan error)
		go func() {
			timestamp, err := publish(ctx, batch)
			if err != nil {
				PlatformHTTPRequestErrors.Inc()
				state.SetPublishState(platformStateDown)

				errCh <- err
				return
			}
			if timestamp != 0 {
				timesync.Refresh(timestamp)
			}
			MetricsPublishedCnt.Add(float64(len(batch)))
			state.SetPublishState(platformStateUp)

			errCh <- nil
		}()

		select {
		case err := <-errCh:
			return err // might be nil
		case <-time.After(30 * time.Second):
			return fmt.Errorf("publish goroutine timeout (30s)")
		}
	}

	return publishFunc
}

type AgentState struct {
	publishState platformState
}

func (a *AgentState) PublishState() platformState {
	return platformState(atomic.LoadInt32((*int32)(&a.publishState)))
}

func (a *AgentState) SetPublishState(st platformState) {
	atomic.StoreInt32((*int32)(&a.publishState), int32(st))
}

func (a *AgentState) Reset() {
	atomic.StoreInt32((*int32)(&a.publishState), int32(platformStateUp))
}

func (h *HTTP) Start(wg *sync.WaitGroup) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, AgentUUIDContextKey, h.conf.UUID)
	ctx = context.WithValue(ctx, PlatformAddrContextKey, h.conf.URL)

	conf := buf.ControllerConf{
		MaxDrainBatchLen: h.conf.MaxBatchLen,
		DrainOp:          h.NewPublishFuncWithContext(ctx),
		DrainFreq:        h.conf.PublishFreq,
	}
	bufCtrl := buf.NewController(conf, h.buffer)

	//
	// start buffer controller
	wg.Add(1)
	go func() {
		defer wg.Done()

		bufCtrl.Start(ctx)
	}()

	//
	// start goroutine for metric ingestion
	wg.Add(1)

	rand.Seed(time.Now().UnixNano())
	go func() {
		defer wg.Done()

		logrus.Debug("[pub] starting metric ingestion")

		var prevErr error
		for {
			select {
			case msg, ok := <-h.receiveCh:
				if !ok {
					logrus.Error("[pub] receive channel closed")

					return
				}

				m, ok := msg.(model.MetricPlatform)
				if !ok {
					logrus.Error("[pub] type assertion failed")

					continue
				}

				item := buf.Item{
					Priority:  0,
					Timestamp: m.Timestamp,
					Bytes:     uint(unsafe.Sizeof(buf.Item{})) + m.Bytes(),
					Data:      m,
				}

				_, err := h.buffer.Insert(item)
				if err != nil {
					MetricsDropCnt.Inc()
					if prevErr == nil {
						logrus.Error("[pub] metric dropped, buffer unavailable: ", err)
						prevErr = err
					}
					continue
				}

				publishState := state.PublishState()
				if publishState == platformStateUp {
					if h.buffer.Len() >= h.conf.MaxBatchLen {
						logrus.Debug("[pub] eager drain kick in")

						drainErr := bufCtrl.Drain()
						if drainErr != nil {
							logrus.Warn("[pub] eager drain error ", drainErr)
							prevErr = drainErr

							continue
						}
						logrus.Debug("[pub] eager drain ok")
					}
				}
				prevErr = nil
			case <-h.closeCh:
				logrus.Debug("[pub] stopping buf controller, ingestion goroutine exit ")
				bufCtrl.Stop()

				return
			}
		}
	}()
}

func (h *HTTP) Stop() {
	close(h.closeCh)
}
