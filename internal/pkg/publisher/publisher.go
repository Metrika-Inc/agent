package publisher

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
	"unsafe"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"go.uber.org/zap"
)

const (
	agentUpTimerFreq      = 30 * time.Second
	defaultPublishTimeout = 5 * time.Second
)

type PublisherConf struct{}

type Publisher struct {
	receiveCh <-chan interface{}

	conf    PublisherConf
	closeCh chan interface{}

	log     *zap.SugaredLogger
	bufCtrl *buf.Controller
	once    sync.Once
	lastErr error
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewPublisher(ch <-chan interface{}, conf PublisherConf, bufCtrl *buf.Controller) *Publisher {
	publisher := &Publisher{
		conf:      conf,
		receiveCh: ch,
		closeCh:   make(chan interface{}),
		log:       zap.S().With("publisher", "transport"),
		bufCtrl:   bufCtrl,
	}

	return publisher
}

func (t *Publisher) forceSendAgentUp(uptime time.Time) {
	log := zap.S()

	t.once.Do(func() {
		agentUpCtx := map[string]interface{}{
			model.AgentUptimeKey:   time.Since(uptime).String(),
			model.AgentProtocolKey: global.BlockchainNode.Protocol(),
			model.AgentVersionKey:  global.Version,
		}
		if err := t.bufCtrl.EmitEvent(agentUpCtx, model.AgentUpName); err != nil {
			log.Warnw("error emitting startup event", "event", model.AgentUpName, zap.Error(err))
		}
		log.Infow("started publishing with agent up", "event_name", model.AgentUpName, "ctx", agentUpCtx)

		// do a manual drain first to send all startup events
		// immediately instead of waiting for first publish
		// interval to pass.
		if err := t.bufCtrl.BufDrain(); err != nil {
			fmt.Println("initial drain error")
			log.Errorw("initial drain error", zap.Error(err))
			t.lastErr = err
		}
	})
}

func (t *Publisher) Start(wg *sync.WaitGroup) {
	log := zap.S()

	agentUpCtx := make(map[string]interface{}, 1)
	agentUpTimer := time.NewTicker(agentUpTimerFreq)
	agentUppedTime := timesync.Now()

	// send agent.up immediately bypassing all buffers
	t.forceSendAgentUp(agentUppedTime)

	// start buffer controller
	wg.Add(1)
	go func() {
		defer wg.Done()

		t.bufCtrl.Start()
	}()

	// start goroutine for metric ingestion
	wg.Add(1)
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

				err := t.bufCtrl.BufInsertAndEarlyDrain(item)
				if err != nil {
					if t.lastErr == nil {
						log.Errorw("buffer insert+drain error", zap.Error(err))
						t.lastErr = err
					}
					continue
				}
				t.lastErr = nil
			case <-agentUpTimer.C:
				agentUpCtx[model.AgentUptimeKey] = time.Since(agentUppedTime).String()
				agentUpCtx[model.AgentProtocolKey] = global.BlockchainNode.Protocol()
				agentUpCtx[model.AgentVersionKey] = global.Version
				if err := t.bufCtrl.EmitEvent(agentUpCtx, model.AgentUpName); err != nil {
					log.Warnw("error emitting event", "event", model.AgentUpName, zap.Error(err))
				}
			case <-t.closeCh:
				log.Debug("stopping buf controller, ingestion goroutine exiting")
				t.bufCtrl.Stop()

				return
			}
		}
	}()
}

func (t *Publisher) Stop() {
	close(t.closeCh)
}
