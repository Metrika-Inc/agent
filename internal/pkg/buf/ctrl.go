package buf

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type ControllerConf struct {
	// MaxDrainBatchLen max number of items to drain at once
	MaxDrainBatchLen int

	// DrainFreq drain frequency
	DrainFreq time.Duration

	// DrainOp callback executed when an batch item is removed from the buffer
	DrainOp func(data ItemBatch) error
}

type Controller struct {
	ControllerConf

	B       Buffer
	closeCh chan bool
}

func NewController(conf ControllerConf, b Buffer) *Controller {
	return &Controller{conf, b, make(chan bool)}
}

func (c *Controller) Start(ctx context.Context) {
	log := zap.S()
	log.Info("Starting buffer controller")

	backof := backoff.NewExponentialBackOff()
	backof.MaxElapsedTime = 0 // never expire

	// no errors, reset to periodic timer
	select {
	case <-time.After(c.DrainFreq):
		backof.Reset()
	case <-c.closeCh:
		c.Drain()

		return
	}

	for {
		log.Debugw("Buffer stats", "buffer_length", c.B.Len(), "buffer_bytes", c.B.Bytes())

		// use exp backoff if errors occur
		log.Debug("Scheduled drain kick in")
		if err := c.Drain(); err != nil {
			nextBo := backof.NextBackOff()
			log.Warnw("Scheduled drain failed", zap.Error(err), "retry_timer", nextBo)

			select {
			case <-time.After(nextBo):
				continue
			case <-c.closeCh:
				c.Drain()

				return
			}
		}

		// no errors, reset to periodic timer
		select {
		case <-time.After(c.DrainFreq):
			backof.Reset()
		case <-c.closeCh:
			c.Drain()

			return
		}

		log.Debug("Scheduled drain ok")
	}
}

func (c *Controller) Stop() {
	close(c.closeCh)
}

func (c *Controller) Drain() error {
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

		if err := c.DrainOp(items); err != nil {
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

	for c.B.Len() > c.MaxDrainBatchLen {
		if err := drainFunc(c.MaxDrainBatchLen); err != nil {
			return err
		}

	}

	batchN := min(c.B.Len(), c.MaxDrainBatchLen)
	if err := drainFunc(batchN); err != nil {
		return err
	}

	if drainedCnt > 0 {
		zap.S().Infow("Buffer drain ok", "item_count", drainedCnt, "drained_size", drainedSz)
	}

	return nil
}
