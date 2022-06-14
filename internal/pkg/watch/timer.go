package watch

import (
	"time"
)

// *** TimerWatch ***

type TimerWatchConf struct {
	Interval time.Duration
}

type TimerWatch struct {
	TimerWatchConf
	Watch
}

func NewTimerWatch(conf TimerWatchConf) *TimerWatch {
	w := new(TimerWatch)
	w.Watch = NewWatch()
	w.TimerWatchConf = conf

	if w.Interval < 1 {
		w.Log.Debug("Using default interval of one second since none was provided.")
		w.Interval = time.Second
	}

	return w
}

func (w *TimerWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	w.wg.Add(1)
	go w.timerLoop()
}

func (w *TimerWatch) timerLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-time.After(w.Interval):
			w.Emit(0)

		case <-w.StopKey:
			return

		}
	}
}
