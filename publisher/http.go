package publisher

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/pkg/timesync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

type TransportConf struct {
	// URL where the publisher will be pushing metrics to.
	URL string

	// UUID the agent's unique identifier
	UUID string

	// Timeout default timeout for requests to the platform
	Timeout time.Duration

	// MaxBatchLen max number of metrics published at once to the platform
	MaxBatchLen int

	// MaxBufferBytes max size of the buffer
	MaxBufferBytes uint

	// PublishIntv is the (periodic) publishing interval
	PublishIntv time.Duration

	// BufferTTL max duration a metric can stay in the buffer
	BufferTTL time.Duration

	// RetryCount attempts at (re)establishing connection
	RetryCount int
}

type Transport struct {
	receiveCh <-chan interface{}

	conf    TransportConf
	client  *http.Client
	buffer  buf.Buffer
	closeCh chan interface{}

	grpcConn     *grpc.ClientConn
	agentService model.AgentClient
}

func NewHTTP(ch <-chan interface{}, conf TransportConf) *Transport {
	state := new(AgentState)
	state.Reset()

	return &Transport{
		client:    http.DefaultClient,
		conf:      conf,
		receiveCh: ch,
		buffer:    buf.NewPriorityBuffer(conf.MaxBufferBytes, conf.BufferTTL),
		closeCh:   make(chan interface{}),
	}
}

func (t *Transport) publish(ctx context.Context, data []*model.Message) (int64, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	uuid, err := stringFromContext(ctx, AgentUUIDContextKey)
	if err != nil {
		return 0, err
	}

	metrikaMsg := model.PlatformMessage{
		AgentUUID: uuid,
		Data:      data,
	}
	if t.agentService == nil {
		if err := t.Connect(); err != nil {
			return 0, err
		}
	}

	// Transmit to platform. Failure here signifies transient error.
	// Try to reconnect and send data again once.
	resp, err := t.agentService.Transmit(reqCtx, &metrikaMsg)
	if err != nil {
		zap.S().Errorw("failed to transmit to the platform", zap.Error(err))
		err := t.Connect()
		if err != nil {
			zap.S().Errorw("failde to reconnect to the platform", zap.Error(err))
			return 0, err
		}
		resp, err = t.agentService.Transmit(reqCtx, &metrikaMsg)
		if err != nil {
			return 0, err
		}
	}
	return resp.Timestamp, nil
}

func (h *Transport) NewPublishFuncWithContext(ctx context.Context) func(b buf.ItemBatch) error {
	publishFunc := func(b buf.ItemBatch) error {
		batch := make([]*model.Message, 0, len(b)+1)
		for _, item := range b {
			m, ok := item.Data.(model.Message)
			if !ok {
				zap.S().Warnf("unrecognised type %T", item.Data)

				// ignore
				continue
			}
			batch = append(batch, &m)
		}

		errCh := make(chan error)
		go func() {
			timestamp, err := h.publish(ctx, batch)
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

func (t *Transport) Connect() error {
	var success bool
	var err error
	for i := 0; i < t.conf.RetryCount && !success; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), t.conf.Timeout)
		defer cancel()
		t.grpcConn, err = grpc.DialContext(ctx, t.conf.URL,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			zap.S().Warnw("failed to connect to GRPC server", "attempt", i+1, zap.Error(err))
			continue
		}
		success = true
	}
	if !success {
		zap.S().Errorw("failed to connect", "retry_count", t.conf.RetryCount, zap.Error(err))
		return err
	}
	t.agentService = model.NewAgentClient(t.grpcConn)
	return nil
}

func (h *Transport) Start(wg *sync.WaitGroup) {
	log := zap.S()
	ctx := context.Background()
	ctx = context.WithValue(ctx, AgentUUIDContextKey, h.conf.UUID)
	ctx = context.WithValue(ctx, PlatformAddrContextKey, h.conf.URL)

	conf := buf.ControllerConf{
		MaxDrainBatchLen: h.conf.MaxBatchLen,
		DrainOp:          h.NewPublishFuncWithContext(ctx),
		DrainFreq:        h.conf.PublishIntv,
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

		log.Debug("starting metric ingestion")

		var prevErr error
		for {
			select {
			case msg, ok := <-h.receiveCh:
				if !ok {
					log.Error("receive channel closed")

					return
				}

				m, ok := msg.(model.Message)
				if !ok {
					log.Error("type assertion failed")

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
						log.Errorw("metric dropped, buffer unavailable", zap.Error(err))
						prevErr = err
					}
					continue
				}

				publishState := state.PublishState()
				if publishState == platformStateUp {
					if h.buffer.Len() >= h.conf.MaxBatchLen {
						log.Debug("maxBatchLen exceeded, eager drain kick in")

						drainErr := bufCtrl.Drain()
						if drainErr != nil {
							log.Warn("eager drain failed", zap.Error(drainErr))
							prevErr = drainErr

							continue
						}
						log.Debug("eager drain ok")
					}
				}
				prevErr = nil
			case <-h.closeCh:
				log.Debug("stopping buf controller, ingestion goroutine exiting")
				bufCtrl.Stop()

				return
			}
		}
	}()
}

func (h *Transport) Stop() {
	close(h.closeCh)
}
