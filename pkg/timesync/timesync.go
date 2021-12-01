package timesync

import (
	"context"
	"sync"
	"time"

	"github.com/beevik/ntp"
	log "github.com/sirupsen/logrus"
)

type TimeSync struct {
	host     string
	ticker   *time.Ticker
	delta    time.Duration
	interval time.Duration
	ctx      context.Context
	wg       *sync.WaitGroup
}

const defaultSyncInterval = 120 * time.Second

func NewTimeSync(host string, ctx context.Context) *TimeSync {
	return &TimeSync{
		host: host,
		ctx:  ctx,
		wg:   &sync.WaitGroup{},
	}
}

func (t *TimeSync) SetSyncInterval(interval time.Duration) {
	t.interval = interval
}

func (t *TimeSync) Start() {
	t.wg.Add(1)
	defer t.wg.Done()

	if t.interval == 0 {
		t.interval = defaultSyncInterval
	}

	// Ensure only single ticker routine is running
	if t.ticker != nil {
		t.ticker.Stop()
	}

	t.ticker = time.NewTicker(t.interval)
	ok := true
	nextTickOrStop := func() {
		select {
		case <-t.ticker.C:
		case <-t.ctx.Done():
			ok = false
		}
	}
	for ; ok; nextTickOrStop() {
		err := t.QueryNTP()
		if err != nil {
			log.Error("Failed to sync time with NTP Server: ", err)
		}
	}
}

func (t *TimeSync) SyncNow() error {
	t.ticker.Reset(t.interval)
	return t.QueryNTP()
}

func (t *TimeSync) QueryNTP() error {
	q, err := ntp.Query(t.host)
	if err != nil {
		return err
	}
	t.delta = q.ClockOffset
	log.Infof("Response: %+v", q)
	log.Infof("NTP time: %v", q.Time)
	log.Infof("System offset: %v", q.ClockOffset)
	return nil
}

func (t *TimeSync) Offset() time.Duration {
	return t.delta
}

func (t *TimeSync) WaitForDone() {
	t.wg.Wait()
}
