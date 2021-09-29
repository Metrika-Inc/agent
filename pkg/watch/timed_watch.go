package watch

import (
	"fmt"
	"time"
)

type TimerWatch struct {
	Watch

	Interval time.Duration
	stop     chan bool
}

func NewTimerWatch(interval time.Duration) *TimerWatch {
	w := new(TimerWatch)
	w.Watch = NewWatch()
	w.StartFn = func() { go w.TimerLoop() }

	w.Interval = interval

	return w
}

func (w *TimerWatch) TimerLoop() {
	for {
		select {
		case <-time.After(w.Interval):
			fmt.Println("TimedWatch/Trigger")
			w.Emit(make([]byte, 0))
		case <-w.stop:
			return
		}
	}
}
