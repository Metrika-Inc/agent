package buf

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	logrus.Info("[bufCtrl] starting buffer controller")

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
		logrus.Debugf("[bufCtrl] buffer stats %d %dB", c.B.Len(), c.B.Bytes())

		// use exp backoff if errors occur
		logrus.Debugf("[bufCtrl] scheduled drain kick in")
		if err := c.Drain(); err != nil {
			nextBo := backof.NextBackOff()
			logrus.Warn("[bufCtrl] error ", err)
			logrus.Warn("[bufCtrl] will retry again in ", nextBo)

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

		logrus.Debugf("[bufCtrl] scheduled drain ok")
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
		logrus.Infof("[bufCtrl] buffer drain ok %d %dB", drainedCnt, drainedSz)
	}

	return nil
}
