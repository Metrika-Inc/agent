package timesync

import (
	"context"
	"sync"
	"time"

	"github.com/beevik/ntp"
	log "github.com/sirupsen/logrus"
)

type TimeSync struct {
	host           string
	ticker         *time.Ticker
	delta          time.Duration
	interval       time.Duration
	ctx, tickerCtx context.Context
	tickerStop     context.CancelFunc
	wg             *sync.WaitGroup
	queryNTP       func(string) (*ntp.Response, error) // to enable mocking
	shouldAdjust   bool
	retries        int
	*sync.RWMutex
}

const (
	defaultSyncInterval = 120 * time.Second
	defaultRetryCount   = 3
)

func NewTimeSync(host string, retries int, ctx context.Context) *TimeSync {
	if retries <= 0 {
		retries = defaultRetryCount
	}
	return &TimeSync{
		host:     host,
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
		queryNTP: ntp.Query,
		retries:  retries,
		RWMutex:  &sync.RWMutex{},
	}
}

// SetSyncInterval sets a new interval for querying the NTP server.
// It does NOT affect the currently running routine. Use Start()
// after SetSyncInterval to update the running routine.
func (t *TimeSync) SetSyncInterval(interval time.Duration) {
	t.interval = interval
}

func (t *TimeSync) Start() {
	go func() {
		t.Lock()
		t.wg.Add(1)
		defer t.wg.Done()

		if t.interval == 0 {
			t.interval = defaultSyncInterval
		}

		// Ensure only single ticker routine is running
		if t.ticker != nil {
			t.stop()
		}

		t.tickerCtx, t.tickerStop = context.WithCancel(context.Background())
		t.ticker = time.NewTicker(t.interval)
		t.Unlock()

	loop:
		for {
			select {
			case <-t.ticker.C:
				err := t.QueryNTP()
				if err != nil {
					log.Error("Failed to sync time with NTP Server: ", err)
					continue
				}
				t.setAdjustBool()
			case <-t.tickerCtx.Done():
				break loop
			case <-t.ctx.Done():
				break loop
			}
		}
	}()
}

// Stop is thread-safe way to stop a current ticker
func (t *TimeSync) Stop() {
	t.Lock()
	defer t.Unlock()
	t.stop()
}

func (t *TimeSync) stop() {
	// stop the time.Ticker
	t.ticker.Stop()
	// make the ticker goroutine exit gracefully
	t.tickerStop()
}

func (t *TimeSync) SyncNow() error {
	t.Lock()
	t.ticker.Reset(t.interval)
	t.Unlock()
	return t.QueryNTP()
}

func (t *TimeSync) QueryNTP() error {
	var resp *ntp.Response
	var err error
	for i := 0; i < t.retries; i++ {
		resp, err = t.queryNTP(t.host)
		if err == nil {
			break
		}
		log.Warnf("Error querying NTP server (attempt %d of %d): %v", i+1, t.retries, err)
	}
	if err != nil {
		return err
	}

	t.Lock()
	t.delta = resp.ClockOffset
	t.Unlock()
	log.Debugf("NTP server response: %+v", resp)
	log.Infof("[NTP] Time synced. System offset: %v", resp.ClockOffset)
	return nil
}

func (t *TimeSync) Offset() time.Duration {
	t.RLock()
	defer t.RUnlock()
	return t.delta
}

func (t *TimeSync) WaitForDone() {
	t.wg.Wait()
}

func (t *TimeSync) setAdjustBool() {
	// TODO: select sensible values for when it's appropriate to adjust timedrift
	t.Lock()
	defer t.Unlock()
	if t.delta > 3*time.Second || t.delta < -500*time.Millisecond {
		t.shouldAdjust = true
	} else {
		t.shouldAdjust = false
	}
}

func (t *TimeSync) Now() time.Time {
	t.RLock()
	defer t.RUnlock()
	if t.shouldAdjust {
		return time.Now().Add(t.delta)
	}
	return time.Now()
}
