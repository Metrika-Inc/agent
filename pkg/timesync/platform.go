package timesync

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	maxDelta      = 20 * time.Second
	subsequentMax = 5 * time.Second
)

type PlatformSync struct {
	firstTimestamp time.Time     // first response from platform
	firstDelta     time.Duration // first difference between platform and local time

	prevTimestamp time.Time // one before last response
	prevDelta     time.Duration

	currentTimestamp time.Time
	currentDelta     time.Duration

	*sync.RWMutex
}

func (p *PlatformSync) Register(ns int64) {
	p.Lock()
	defer p.Unlock()
	ts := time.Unix(0, ns)
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

}

func (p *PlatformSync) Healthy() bool {
	p.RLock()
	difference := abs(p.currentDelta - p.prevDelta)
	p.RUnlock()

	if difference > subsequentMax || p.currentDelta > maxDelta {
		logrus.Warn("[timesync] we are behind")
		return false
	}
	return true
}

func (p *PlatformSync) LastDeltas() (time.Duration, time.Duration) {
	p.RLock()
	defer p.RUnlock()
	return p.prevDelta, p.currentDelta
}

func (p *PlatformSync) Clear() {
	p.Lock()
	defer p.Unlock()
	p.firstTimestamp = time.Time{}
}

func Register(ts int64) {
	Default.Register(ts)
}

func Healthy() bool {
	return Default.Healthy()
}

func RegisterAndCheck(ts int64) bool {
	Default.Register(ts)
	return Default.Healthy()
}

func LastDeltas() (time.Duration, time.Duration) {
	return Default.LastDeltas()
}

func Clear() {
	Default.Clear()
}

func TrackTimestamps(ctx context.Context) chan<- int64 {
	c := make(chan int64, 5)
	go func() {
		for {
			select {
			case timestamp := <-c:
				if ok := RegisterAndCheck(timestamp); !ok {
					prevDelta, currDelta := LastDeltas()
					logrus.Warnf("Delta between platform and local time passed the threshold. Prev: %v, curr: %v", prevDelta, currDelta)
					if err := Default.SyncNow(); err != nil {
						logrus.Error("failed to resync: ", err)
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
