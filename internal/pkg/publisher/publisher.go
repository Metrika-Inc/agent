package publisher

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

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

func NewPublisher(conf PublisherConf, bufCtrl *buf.Controller) *Publisher {
	publisher := &Publisher{
		conf:    conf,
		closeCh: make(chan interface{}),
		log:     zap.S().With("publisher", "transport"),
		bufCtrl: bufCtrl,
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
			case <-agentUpTimer.C:
				// publish periodic update
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

// HandleMessage stores the message into the buffer and checks if buffer is ready
// to be drained. Implements global.Exporter interface.
func (t *Publisher) HandleMessage(ctx context.Context, message *model.Message) {
	item := buf.Item{
		Priority:  0,
		Timestamp: timesync.Now().UnixMilli(),
		Data:      message,
	}

	err := t.bufCtrl.BufInsertAndEarlyDrain(item)
	if err != nil {
		if t.lastErr == nil {
			zap.S().Errorw("buffer insert+drain error", zap.Error(err))
			t.lastErr = err
		}
		return
	}
	t.lastErr = nil
}

func (t *Publisher) Stop() {
	close(t.closeCh)
}
