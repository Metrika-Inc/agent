package buf

import (
	"runtime"
	"time"
	"unsafe"

	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	defaultMaxDrainBatchLen     = 1000
	defaultMaxHeapAllocBytes    = 52428800
	defaultDrainFreq            = 5 * time.Second
	defaultMemStatsCacheTimeout = 15 * time.Second
)

type ControllerConf struct {
	// BufLenLimit max number of items to drain at once
	BufLenLimit int

	// BufDrainFreq drain frequency
	BufDrainFreq time.Duration

	// OnBufRemoveCallback callback executed when an batch item is removed from the buffer.
	OnBufRemoveCallback func(data ItemBatch) error

	// MemStatsCacheTimeout how often to refresh memstats cache
	MemStatsCacheTimeout time.Duration

	// MaxHeapAlloc max allowed heap allocated bytes before the controller
	// starts dropping metrics
	MaxHeapAllocBytes uint64
}

// Controller starts a goroutine to perform periodic buffer cleanup. It also exposes
// a coarse interface to the buffer so that callers can access it.
type Controller struct {
	ControllerConf

	// B Buffer implementation to use for all cached data
	B Buffer

	closeCh chan bool

	// memstats used to cache latest runtime memstats
	memstats          *runtime.MemStats
	memstatsUpdatedAt time.Time
}

func NewController(conf ControllerConf, b Buffer) *Controller {
	if conf.BufLenLimit == 0 {
		conf.BufLenLimit = defaultMaxDrainBatchLen
	}

	if conf.BufDrainFreq == 0 {
		conf.BufDrainFreq = defaultDrainFreq
	}

	if conf.MemStatsCacheTimeout == 0 {
		conf.MemStatsCacheTimeout = defaultMemStatsCacheTimeout
	}

	if conf.MaxHeapAllocBytes == 0 {
		conf.MaxHeapAllocBytes = defaultMaxHeapAllocBytes
	}

	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)

	return &Controller{
		ControllerConf:    conf,
		B:                 b,
		closeCh:           make(chan bool),
		memstats:          &memstats,
		memstatsUpdatedAt: time.Now(),
	}
}

var HeapAllocLimitError = errors.New("heap allocated bytes limit reached")

// Start starts a goroutine that drains the underlying buffer based
// on two timers: by default periodically and falls back to exponential
// backoff if an error occurs while draining. When it recovers, the
// goroutine switches back to periodic drains.
func (c *Controller) Start() {
	log := zap.S()
	log.Debug("starting buffer controller")

	backof := backoff.NewExponentialBackOff()
	backof.MaxElapsedTime = 0 // never expire

	select {
	case <-time.After(c.BufDrainFreq):
		backof.Reset()
	case <-c.closeCh:
		c.BufDrain()

		return
	}

	for {
		// use exp backoff if errors occur
		log.Debug("scheduled drain kick in")
		if err := c.BufDrain(); err != nil {
			nextBo := backof.NextBackOff()
			log.Warnw("scheduled drain failed", zap.Error(err), "retry_timer", nextBo)

			select {
			case <-time.After(nextBo):
				continue
			case <-c.closeCh:
				c.BufDrain()

				return
			}
		}

		// no errors, reset to periodic timer
		select {
		case <-time.After(c.BufDrainFreq):
			backof.Reset()
		case <-c.closeCh:
			c.BufDrain()

			return
		}

		log.Debug("scheduled drain ok")
		log.Debugw("buffer stats", "buffer_length", c.B.Len(), "buffer_bytes", c.B.Bytes())
	}
}

func (c *Controller) Stop() {
	close(c.closeCh)
}

// checkMemStats refreshes memstats if they expired and checks
// the heap allocated objects have not exceeded MaxHeapAllocBytes.
func (c *Controller) checkMemStats() error {
	if time.Since(c.memstatsUpdatedAt) > c.MemStatsCacheTimeout {
		runtime.ReadMemStats(c.memstats)
		c.memstatsUpdatedAt = time.Now()
	}

	if c.memstats.HeapAlloc > c.MaxHeapAllocBytes {
		return HeapAllocLimitError
	}

	return nil
}

// BufInsert inserts an item in the backing buffer.
func (c *Controller) BufInsert(item Item) error {
	if err := c.checkMemStats(); err != nil {
		MetricsDropCnt.WithLabelValues("memstats_error").Inc()

		return err
	}

	_, err := c.B.Insert(item)
	if err != nil {
		MetricsDropCnt.WithLabelValues("buffer_full").Inc()

		return err
	}

	return nil
}

// BufInsertAndEarlyDrain inserts an item to the underlying buffer and also checks if
// it should be drained based on the current number of bufferred items. If platform state
// is down, it just buffers the item and returns nil.
func (c *Controller) BufInsertAndEarlyDrain(item Item) error {
	if err := c.BufInsert(item); err != nil {
		return err
	}

	publishState := global.AgentRuntimeState.PublishState()
	if publishState == global.PlatformStateUp {
		if c.B.Len() >= c.BufLenLimit {
			zap.S().Debug("maxBatchLen exceeded, eager drain kick in")

			drainErr := c.BufDrain()
			if drainErr != nil {
				zap.S().Warnw("eager drain failed", zap.Error(drainErr))

				return drainErr
			}
			zap.S().Debug("eager drain ok")
		}
	}
	return nil
}

// BufDrain empties the buffer and executes remove callbacks for each batch
// removed from the buffer. If callback returns an error try to put the batch
// back to the buffer. If inserting back to the buffer fails, it will return an
// error.
func (c *Controller) BufDrain() error {
	t := time.Now()
	defer func() { BufferDrainDuration.Observe(time.Since(t).Seconds()) }()

	drainedCnt := 0
	drainedSz := 0

	drainFunc := func(batchN int) error {
		if batchN < 1 {
			return nil
		}

		items, n, err := c.B.Get(batchN)
		if err != nil {
			return err
		}

		if err := c.OnBufRemoveCallback(items); err != nil {
			if err := c.EmitEventWithError(err, model.AgentNetErrorName); err != nil {
				zap.S().Warnw("error emitting event", "event", model.AgentNetErrorName, zap.Error(err))
			}

			_, inErr := c.B.Insert(items...)
			if inErr != nil {
				err = errors.Wrap(err, inErr.Error())
			}

			return err
		}

		drainedSz += int(n)
		drainedCnt += len(items)

		return nil
	}

	for c.B.Len() > c.BufLenLimit {
		if err := drainFunc(c.BufLenLimit); err != nil {
			return err
		}
	}

	batchN := min(c.B.Len(), c.BufLenLimit)
	if err := drainFunc(batchN); err != nil {
		return err
	}

	if drainedCnt > 0 {
		zap.S().Infow("buffer drain ok", "item_count", drainedCnt, "drained_size", drainedSz)
	}

	return nil
}

func (c *Controller) EmitEventWithError(err error, name string) error {
	ctx := map[string]interface{}{model.ErrorKey: err.Error()}
	return c.EmitEvent(ctx, name)
}

func (c *Controller) EmitEvent(ctx map[string]interface{}, name string) error {
	ev, err := model.NewWithCtx(ctx, name, timesync.Now())
	if err != nil {
		return err
	}

	zap.S().Debugf("emitting event: %s, %v", ev.Name, ev.Values.String())

	m := &model.Message{
		Name:  ev.GetName(),
		Value: &model.Message_Event{Event: ev},
	}

	item := Item{
		Priority: 0,
		Bytes:    uint(unsafe.Sizeof(Item{})) + m.Bytes(),
		Data:     m,
	}

	if err := c.BufInsert(item); err != nil {
		zap.S().Errorw("controller returned buffer insert error (for event)", zap.Error(err))

		return err
	}

	return nil
}
