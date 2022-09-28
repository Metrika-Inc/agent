// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package timesync

import (
	"context"
	"sync"
	"time"

	"agent/api/v1/model"

	"agent/internal/pkg/emit"

	"github.com/beevik/ntp"
	"go.uber.org/zap"
)

// TimeSync maintains a delta between upstream NTP and
// the host's clock and exports methods equivalent to
// the standard time package (i.e. Now()).
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

// Default is the default timesync instance used throughout
// the agent.
var Default = NewTimeSync(context.Background(), "pool.ntp.org", 0)

// NewTimeSync constructor
func NewTimeSync(ctx context.Context, host string, retries int) *TimeSync {
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

// Start starts a goroutine to periodically adjust the agent clock
// according to upstream NTP servers.
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

	var lasterr error
	go func() {
		defer t.wg.Done()
		defer t.tickerLock.Unlock()
		for {
			select {
			case <-t.ticker.C:
				err := t.QueryNTP()
				if err != nil {
					ctx := map[string]interface{}{model.ErrorKey: err.Error()}
					EmitEventWithCtx(t, ctx, model.AgentClockNoSyncName)

					zap.S().Errorw("failed to sync time with NTP server", zap.Error(err))

					lasterr = err
					continue
				}

				if lasterr != nil {
					EmitEvent(t, model.AgentClockSyncName)
					lasterr = err
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

// Offset returns the current NTP ClockOffset (see ntp docs)
func (t *TimeSync) Offset() time.Duration {
	t.RLock()
	defer t.RUnlock()
	return t.delta
}

func (t *TimeSync) waitForDone() {
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

// EmitEvent emits an event related to the timesync package.
func EmitEvent(t *TimeSync, name string) error {
	return EmitEventWithCtx(t, nil, name)
}

// EmitEventWithCtx emits events with context related to the timesync package.
func EmitEventWithCtx(t *TimeSync, ctx map[string]interface{}, name string) error {
	t.RLock()
	defaultCtx := map[string]interface{}{
		model.NTPServerKey:    t.host,
		model.OffsetMillisKey: time.Nanosecond * t.delta / time.Millisecond,
	}
	t.RUnlock()

	if ctx != nil {
		// merge default and given contexts
		for key, val := range ctx {
			defaultCtx[key] = val
		}
	}
	ev, err := model.NewWithCtx(ctx, name, t.Now())
	if err != nil {
		return err
	}

	if err := emit.Ev(t, ev); err != nil {
		return err
	}

	return nil
}

// Emit emits a timesync message to the configured channel.
func (t *TimeSync) Emit(message interface{}) {
	if t.emitch == nil {
		zap.S().Warn("emit channel is not configured")

		return
	}

	t.emitch <- message
}
