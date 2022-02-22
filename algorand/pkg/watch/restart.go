package watch

import (
	"agent/api/v1/model"
	"agent/pkg/timesync"
	. "agent/pkg/watch"
)

type AlgodRestartWatchConf struct {
	Path string // Should be resolved from Algorand data directory
}

type AlgodRestartWatch struct {
	AlgodRestartWatchConf
	Watch

	PidWatch Watcher
	pidCh    chan interface{}
}

func NewAlgodRestartWatch(conf AlgodRestartWatchConf, pidWatch Watcher) *AlgodRestartWatch {
	w := new(AlgodRestartWatch)
	w.AlgodRestartWatchConf = conf
	w.Watch = NewWatch()
	w.PidWatch = pidWatch
	w.pidCh = make(chan interface{}, 1)
	return w
}

func (w *AlgodRestartWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	//todo path validation

	// Start the PID watch
	if w.PidWatch == nil {
		w.PidWatch = NewDotPidWatch(DotPidWatchConf{Path: w.Path})
	}
	w.PidWatch.Subscribe(w.pidCh)
	go w.handlePidChange()

	Start(w.PidWatch)
}

func (w *AlgodRestartWatch) Stop() {
	w.Watch.Stop()

	w.PidWatch.Stop()
}

func (w *AlgodRestartWatch) handlePidChange() {
	for {
		select {
		case message := <-w.pidCh:
			newPid := message.(int)

			// Create health struct
			var health model.NodeState
			if newPid != 0 { // On
				health = model.NodeState_up
			} else { // Off
				health = model.NodeState_down
			}

			// Create & emit the event (empty body; all needed info is in NodeState)
			metric := model.Message{
				Timestamp: timesync.Now().UnixMilli(),
				Name:      "node.health",
				Type:      model.MessageType_event,
				NodeState: health,
			}
			w.Emit(metric)

		case <-w.StopKey:
			return
		}
	}
}
