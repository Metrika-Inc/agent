package timesync

import (
	"agent/api/v1/model"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
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
	emitch         chan<- interface{}
	tickerLock     *sync.Mutex
	*PlatformSync
	*sync.RWMutex
}

const (
	defaultSyncInterval = 120 * time.Second
	defaultRetryCount   = 3
)

var Default = NewTimeSync("pool.ntp.org", 0, context.Background())

func NewTimeSync(host string, retries int, ctx context.Context) *TimeSync {
	if retries <= 0 {
		retries = defaultRetryCount
	}
	return &TimeSync{
		host:         host,
		ctx:          ctx,
		wg:           &sync.WaitGroup{},
		queryNTP:     ntp.Query,
		retries:      retries,
		RWMutex:      &sync.RWMutex{},
		tickerLock:   &sync.Mutex{},
		PlatformSync: &PlatformSync{RWMutex: &sync.RWMutex{}},
	}
}

// SetSyncInterval sets a new interval for querying the NTP server.
// It does NOT affect the currently running routine. Use Start()
// after SetSyncInterval to update the running routine.
func (t *TimeSync) SetSyncInterval(interval time.Duration) {
	t.interval = interval
}

func (t *TimeSync) Start(emitch chan<- interface{}) {
	t.Lock()
	t.wg.Add(1)

	if t.interval == 0 {
		t.interval = defaultSyncInterval
	}

	// Ensure only single ticker routine is running
	if t.ticker != nil {
		t.stop()
	}

	t.tickerLock.Lock()

	t.tickerCtx, t.tickerStop = context.WithCancel(context.Background())
	t.ticker = time.NewTicker(t.interval)
	t.emitch = emitch
	t.Unlock()

	// Setup PlatformSync
	t.Listen()

	newEvCtx := func(ctx map[string]interface{}) map[string]interface{} {
		t.RLock()
		defaultCtx := map[string]interface{}{
			"ntp_server":    t.host,
			"offset_millis": time.Nanosecond * t.delta / time.Millisecond,
		}
		t.RUnlock()

		if ctx != nil {
			// merge default and given contexts
			for key, val := range ctx {
				defaultCtx[key] = val
			}
		}

		return defaultCtx
	}

	lasterr := errors.New("")
	go func() {
		defer t.wg.Done()
		defer t.tickerLock.Unlock()
		for {
			select {
			case <-t.ticker.C:
				err := t.QueryNTP()
				if err != nil {
					ctx := newEvCtx(map[string]interface{}{"error": err.Error()})
					EmitEventWithCtx(t, ctx, model.AgentClockNoSyncName, model.AgentClockNoSyncDesc)

					zap.S().Errorw("failed to sync time with NTP server", zap.Error(err))
					continue
				}
				if lasterr != nil {
					EmitEventWithCtx(t, newEvCtx(nil), model.AgentClockSyncName, model.AgentClockSyncDesc)
					lasterr = nil
				}
				t.setAdjustBool()
			case <-t.tickerCtx.Done():
				return
			case <-t.ctx.Done():
				return
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

// SyncNow queries the NTP server right away and resets
// the periodic timer.
func (t *TimeSync) SyncNow() error {
	t.Lock()
	t.ticker.Reset(t.interval)
	t.Unlock()
	return t.QueryNTP()
}

// QueryNTP queries the NTP server at address of TimeSync.host
// It stores the clock offset in TimeSync.delta.
func (t *TimeSync) QueryNTP() error {
	log := zap.S()
	var resp *ntp.Response
	var err error
	for i := 0; i < t.retries; i++ {
		resp, err = t.queryNTP(t.host)
		if err == nil {
			break
		}
		log.Warnw("error querying NTP server", "attempt", i+1, zap.Error(err))
	}
	if err != nil {
		return err
	}

	t.Lock()
	t.delta = resp.ClockOffset
	t.Unlock()
	log.Infow("time synced with NTP server", "clock_offset", resp.ClockOffset)
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

// Now returns the current time. May be adjusted
// based on the offset received from NTP server.
func (t *TimeSync) Now() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.now()
}

// NewTicker implements zapcore.Clock interface.
func (t *TimeSync) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

func (t *TimeSync) now() time.Time {
	if t.shouldAdjust {
		return time.Now().Add(t.delta)
	}
	return time.Now()
}

// Now is a convenience wrapper for calling timesync.Default.Now()
func Now() time.Time {
	Default.RLock()
	defer Default.RUnlock()
	return Default.now()
}

func EmitEvent(t *TimeSync, name, desc string) error {
	return EmitEventWithCtx(t, nil, name, desc)
}

func EmitEventWithCtx(t *TimeSync, ctx map[string]interface{}, name, desc string) error {
	t.RLock()
	defaultCtx := map[string]interface{}{
		"ntp_server":    t.host,
		"offset_millis": time.Nanosecond * t.delta / time.Millisecond,
	}
	t.RUnlock()

	if ctx != nil {
		// merge default and given contexts
		for key, val := range ctx {
			defaultCtx[key] = val
		}
	}
	ev := model.NewWithCtx(ctx, name, desc)
	zap.S().Debugf("emitting event: %s, %v", ev.Name, string(ev.Values))

	evBytes, err := proto.Marshal(ev)
	if err != nil {
		return err
	}

	m := model.Message{
		Type:      model.MessageType_event,
		Timestamp: Now().Unix(),
		Body:      evBytes,
	}

	if t.emitch == nil {
		return fmt.Errorf("nil emit channel")
	}

	t.emitch <- m

	return nil
}
