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

	"go.uber.org/zap"
)

const (
	maxDelta      = 20 * time.Second
	subsequentMax = 5 * time.Second
)

// PlatformSync keeps track of Metrika Platform time.
// It calculates deltas by comparing local time with Platform time.
// When receiving a new timestamp, it is checked against the previous and first timestamps.
//
// Checking against previous timestamp allows the agent to know whether  it is still in sync.
// It could possibly go out of sync due to VM pauses, migrations etc. (sudden change in delta)
//
// Checking against first timestamp allows the agent to account for a possible clock drift, when
// delta changes very slowly, yet consistently.
type PlatformSync struct {
	firstTimestamp time.Time     // first response from platform
	firstDelta     time.Duration // first difference between platform and local time

	prevTimestamp time.Time // one before last response
	prevDelta     time.Duration

	currentTimestamp time.Time
	currentDelta     time.Duration

	tsChan chan<- int64

	*sync.RWMutex
}

// Register takes in the new timestamp.
func (p *PlatformSync) Register(ns int64) {
	p.Lock()
	defer p.Unlock()
	ts := time.UnixMilli(ns)
	d := abs(Now().Sub(ts))
	// first measurement, only store
	if p.firstTimestamp.IsZero() {
		p.firstTimestamp = ts
		p.firstDelta = d

		p.prevTimestamp = ts
		p.prevDelta = d

		p.currentTimestamp = ts
		p.currentDelta = d
		return
	}
	p.prevTimestamp = p.currentTimestamp
	p.prevDelta = p.currentDelta
	p.currentTimestamp = ts
	p.currentDelta = d
	zap.S().Debugw("received new timestamp information",
		"current_timestamp", p.currentTimestamp.String(), "current_delta", p.currentDelta.String())
}

// Healthy calculates if agent time is in sync with Metrika Platform
func (p *PlatformSync) Healthy() bool {
	p.RLock()
	difference := abs(p.currentDelta - p.prevDelta)
	p.RUnlock()

	behind := difference > subsequentMax || p.currentDelta > maxDelta
	log := zap.S()
	log.Debugw("timesync health check", "healthy", !behind)
	if behind {
		log.Warnw("we are out of sync")
		return false
	}
	return true
}

// LastDeltas returns last two registered deltas in order of (earlier, latest).
func (p *PlatformSync) LastDeltas() (time.Duration, time.Duration) {
	p.RLock()
	defer p.RUnlock()
	return p.prevDelta, p.currentDelta
}

// Clear clears the first timestamp. That way next incoming timestamp
// will override and previously saved data.
func (p *PlatformSync) Clear() {
	p.Lock()
	defer p.Unlock()
	p.firstTimestamp = time.Time{}
}

// Listen instantiates the goroutine to listen to timestamps from the platform
// in case it was not yet instantiated.
func (p *PlatformSync) Listen() {
	p.Lock()
	defer p.Unlock()
	if Default.tsChan == nil {
		Default.tsChan = TrackTimestamps(Default.ctx)
	}
}

// Register is a convenience wrapper for calling timesync.Default.Register()
func Register(ts int64) {
	Default.Register(ts)
}

// Healthy is a convenience wrapper for calling timesync.Default.Healthy()
func Healthy() bool {
	return Default.Healthy()
}

// RegisterAndCheck is a convenience wrapper for calling timesync.Register()
// and timesync.Healthy() simultaneously.
func RegisterAndCheck(ts int64) bool {
	Default.Register(ts)
	return Default.Healthy()
}

// LastDeltas is a convenience wrapper for calling timesync.Default.LastDeltas()
func LastDeltas() (time.Duration, time.Duration) {
	return Default.LastDeltas()
}

// Clear is a convenience wrapper for calling timesync.Default.Clear()
func Clear() {
	Default.Clear()
}

// Refresh takes in a timestamp and forwards it to the timesync.tsChan., where it gets processed
// and determines the time offset from the platform.
func Refresh(ts int64) {
	Default.tsChan <- ts
}

// Listen is a convenience wrapper for calling timesync.Default.Listen()
func Listen() {
	Default.Listen()
}

// TrackTimestamps returns a channel for sending the incoming timestamps.
// Upon receiving one, it checks if the agent is still in sync with the Platform,
// and if not, calls for a Sync with NTP server (which will correct the agent time).
func TrackTimestamps(ctx context.Context) chan<- int64 {
	c := make(chan int64, 5)
	go func() {
		for {
			select {
			case timestamp := <-c:
				if ok := RegisterAndCheck(timestamp); !ok {
					log := zap.S()
					prevDelta, currDelta := LastDeltas()
					log.Warnw("delta between platform and local time passed the threshold",
						"previous_delta", prevDelta, "current_delta", currDelta)
					if err := Default.SyncNow(); err != nil {
						log.Errorw("failed to resync", zap.Error(err))
					} else {
						Clear()
					}
				}
			case <-ctx.Done():
				close(c)
				return
			}
		}
	}()
	return c
}

func abs(i time.Duration) time.Duration {
	if i < 0 {
		return -i
	}
	return i
}
