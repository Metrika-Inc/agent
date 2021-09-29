package watch

import (
	"fmt"
	"time"
)

type PidWatch struct {
	Watch

	Timer *TimerWatch

	lastPid int
}

func NewPidWatch(timer *TimerWatch) *PidWatch {
	w := new(PidWatch)
	w.Watch = NewWatch()
	w.StartFn = w.Start
	w.StopFn = w.Stop

	w.Timer = timer
	w.lastPid = -1

	return w
}

func (w *PidWatch) Start() {
	if w.Timer == nil {
		w.Timer = NewTimerWatch(1 * time.Second)
	}

	w.Timer.Start()
	w.Timer.Subscribe(w.HandleTimerMessage)
}

func (w *PidWatch) Stop() {
	w.Timer.Stop()
}

func (w *PidWatch) HandleTimerMessage(_ interface{}) {
	fmt.Println("PidWatch/Trigger")
}
