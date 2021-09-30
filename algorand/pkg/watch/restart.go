package watch

import (
	"agent/api/v1/model"
	. "agent/pkg/watch"
	"agent/publisher"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
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
			fmt.Println("New pid: ")
			newPid := message.(int)

			// Create health struct
			var health model.NodeHealthMetric
			if newPid != 0 { // On
				health = model.NodeHealthMetric{
					Metric: model.NewMetric(true),
					State:  model.NodeStateUp,
				}
			} else { // Off
				health = model.NodeHealthMetric{
					Metric: model.NewMetric(true),
					State:  model.NodeStateDown,
				}
			}
			jsonHealth, err := json.Marshal(health)
			if err != nil {
				log.Errorln("[AlgodRestartWatch] failed to marshall node health info: ", err)
				continue
			}

			// Create & emit the metric
			metric := publisher.Metric{
				Type: "node.health",
				Body: jsonHealth,
			}
			w.Emit(metric)

		case <-w.StopKey:
			return
		}
	}
}
