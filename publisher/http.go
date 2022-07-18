package publisher

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type platformState int32

var (
	state *AgentState

	// AgentUUIDHeaderName GRPC metadata key name for agent hostname
	AgentUUIDHeaderName = "x-agent-uuid"

	// AgentAPIKeyHeaderName GRPC metadata key name for platform API key
	AgentAPIKeyHeaderName = "x-api-key"
)

func init() {
	state = new(AgentState)
	state.Reset()
}

const (
	platformStateUknown   platformState = iota
	platformStateUp                     = iota
	platformStateDown                   = iota
	agentUpTimerFreq                    = 30 * time.Second
	defaultPublishTimeout               = 5 * time.Second
)

type TransportConf struct {
	// URL where the publisher will be pushing metrics to.
	URL string

	// UUID the agent's unique identifier
	UUID string

	// APIKey authentication key for the Metrika platform
	APIKey string

	// Timeout default timeout for requests to the platform
	Timeout time.Duration

	// MaxBatchLen max number of metrics published at once to the platform
	MaxBatchLen int

	// MaxBufferBytes max size of the buffer
	MaxBufferBytes uint

	// PublishIntv is the (periodic) publishing interval
	PublishIntv time.Duration

	// PublishTimeout platform publishing timeout
	PublishTimeout time.Duration

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
	metadata     metadata.MD
	log          *zap.SugaredLogger
}

func NewTransport(ch <-chan interface{}, conf TransportConf) *Transport {
	state := new(AgentState)
	state.Reset()

	// Anything put here will be transmitted as request headers.
	md := metadata.Pairs(AgentUUIDHeaderName, conf.UUID, AgentAPIKeyHeaderName, conf.APIKey)

	if conf.PublishTimeout == 0 {
		conf.PublishTimeout = defaultPublishTimeout
	}

	return &Transport{
		client:    http.DefaultClient,
		conf:      conf,
		receiveCh: ch,
		buffer:    buf.NewPriorityBuffer(conf.MaxBufferBytes, conf.BufferTTL),
		closeCh:   make(chan interface{}),
		metadata:  md,
		log:       zap.S().With("publisher", "transport"),
	}
}

func (t *Transport) publish(reqCtx context.Context, data []*model.Message) (int64, error) {
	ctx, cancel := context.WithTimeout(reqCtx, 30*time.Second)
	defer cancel()

	uuid, err := stringFromContext(reqCtx, AgentUUIDContextKey)
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

	ctx = metadata.NewOutgoingContext(ctx, t.metadata)

	// Transmit to platform. Failure here signifies transient error.
	resp, err := t.agentService.Transmit(ctx, &metrikaMsg)
	if err != nil {
		t.log.Errorw("failed to transmit to the platform", zap.Error(err), "addr", t.conf.URL)
		if err := emitEventWithError(t, err, model.AgentNetErrorName); err != nil {
			t.log.Warnw("error emitting event", "event", model.AgentNetErrorName, zap.Error(err))
		}

		// mark service for repair
		t.agentService = nil

		return 0, err
	}

	return resp.Timestamp, nil
}

func (t *Transport) NewPublishFuncWithContext(ctx context.Context) func(b buf.ItemBatch) error {
	publishFunc := func(b buf.ItemBatch) error {
		batch := make([]*model.Message, 0, len(b)+1)
		for _, item := range b {
			m, ok := item.Data.(*model.Message)
			if !ok {
				t.log.Warnf("unrecognised type %T", item.Data)

				// ignore
				continue
			}
			batch = append(batch, m)
		}

		errCh := make(chan error)
		go func() {
			timestamp, err := t.publish(ctx, batch)
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
		case <-time.After(t.conf.PublishTimeout):
			return fmt.Errorf("publish goroutine timeout (%v)", t.conf.PublishTimeout.Seconds())
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
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), t.conf.Timeout)
	defer cancel()

	tlsConfig := &tls.Config{InsecureSkipVerify: false}
	t.grpcConn, err = grpc.DialContext(ctx, t.conf.URL,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithBlock())
	if err != nil {
		emitEventWithError(t, err, model.AgentNetErrorName)
		return err
	}

	t.agentService = model.NewAgentClient(t.grpcConn)

	return nil
}

func (t *Transport) Start(wg *sync.WaitGroup) {
	log := zap.S()

	agentUpCtx := make(map[string]interface{}, 1)
	agentUpCtx[model.AgentProtocolKey] = global.BlockchainNode.Protocol()
	if err := emitEvent(t, agentUpCtx, model.AgentUpName); err != nil {
		log.Warnw("error emitting startup event", "event", model.AgentUpName, zap.Error(err))
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, AgentUUIDContextKey, t.conf.UUID)
	ctx = context.WithValue(ctx, PlatformAddrContextKey, t.conf.URL)

	conf := buf.ControllerConf{
		MaxDrainBatchLen: t.conf.MaxBatchLen,
		DrainOp:          t.NewPublishFuncWithContext(ctx),
		DrainFreq:        t.conf.PublishIntv,
	}

	bufCtrl := buf.NewController(conf, t.buffer)

	// do a manual drain first to send all startup events
	// immediately instead of waiting for first publish
	// interval to pass.
	var lastErr error
	if err := bufCtrl.Drain(); err != nil {
		log.Errorw("initial drain error", zap.Error(err))
		lastErr = err
	}

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
	agentUpTimer := time.NewTicker(agentUpTimerFreq)
	agentUppedTime := timesync.Now()
	go func() {
		defer wg.Done()

		log.Debug("starting metric ingestion")

		for {
			select {
			case msg, ok := <-t.receiveCh:
				if !ok {
					log.Error("receive channel closed")

					return
				}

				m, ok := msg.(*model.Message)
				if !ok {
					log.Error("type assertion failed")

					continue
				}

				item := buf.Item{
					Priority:  0,
					Timestamp: timesync.Now().UnixMilli(),
					Bytes:     uint(unsafe.Sizeof(buf.Item{})) + m.Bytes(),
					Data:      m,
				}

				_, err := t.buffer.Insert(item)
				if err != nil {
					MetricsDropCnt.Inc()
					if lastErr == nil {
						log.Errorw("metric dropped, buffer unavailable", zap.Error(err))
						lastErr = err
					}
					continue
				}

				publishState := state.PublishState()
				if publishState == platformStateUp {
					if t.buffer.Len() >= t.conf.MaxBatchLen {
						log.Debug("maxBatchLen exceeded, eager drain kick in")

						drainErr := bufCtrl.Drain()
						if drainErr != nil {
							log.Warn("eager drain failed", zap.Error(drainErr))
							lastErr = drainErr

							continue
						}
						log.Debug("eager drain ok")
					}
				}
				lastErr = nil
			case <-agentUpTimer.C:
				agentUpCtx[model.AgentUptimeKey] = time.Since(agentUppedTime).String()
				agentUpCtx[model.AgentProtocolKey] = global.BlockchainNode.Protocol()
				if err := emitEvent(t, agentUpCtx, model.AgentUpName); err != nil {
					log.Warnw("error emitting event", "event", model.AgentUpName, zap.Error(err))
				}
			case <-t.closeCh:
				log.Debug("stopping buf controller, ingestion goroutine exiting")
				bufCtrl.Stop()

				return
			}
		}
	}()
}

func emitEventWithError(t *Transport, err error, name string) error {
	ctx := map[string]interface{}{model.ErrorKey: err.Error()}
	return emitEvent(t, ctx, name)
}

func emitEvent(t *Transport, ctx map[string]interface{}, name string) error {
	ev, err := model.NewWithCtx(ctx, name, timesync.Now())
	if err != nil {
		return err
	}

	t.log.Debugf("emitting event: %s, %v", ev.Name, ev.Values.String())

	m := &model.Message{
		Name:  ev.GetName(),
		Value: &model.Message_Event{Event: ev},
	}

	item := buf.Item{
		Priority: 0,
		Bytes:    uint(unsafe.Sizeof(buf.Item{})) + m.Bytes(),
		Data:     m,
	}

	_, err = t.buffer.Insert(item)

	if err != nil {
		MetricsDropCnt.Inc()
		t.log.Errorw("metric dropped, buffer unavailable", zap.Error(err))
		return err
	}

	return nil
}

func (t *Transport) Stop() {
	close(t.closeCh)
}
