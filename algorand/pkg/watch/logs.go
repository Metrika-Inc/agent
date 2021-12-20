package watch

import (
	"encoding/json"

	. "agent/pkg/watch"

	log "github.com/sirupsen/logrus"
)

// *** AlgorandLogWatch ***

type AlgorandLogWatchConf struct {
	Path string
}

type AlgorandLogWatch struct {
	AlgorandLogWatchConf
	Watch

	LogWatch   Watcher
	logWatchCh chan interface{}
}

func NewAlgorandLogWatch(conf AlgorandLogWatchConf, jsonLogWatch Watcher) *AlgorandLogWatch {
	w := new(AlgorandLogWatch)
	w.AlgorandLogWatchConf = conf
	w.Watch = NewWatch()
	w.LogWatch = jsonLogWatch
	w.logWatchCh = make(chan interface{}, 1)
	return w
}

func (w *AlgorandLogWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	if w.LogWatch == nil {
		w.LogWatch = NewJsonLogWatch(JsonLogWatchConf{Path: w.Path}, nil)
	}

	w.LogWatch.Subscribe(w.logWatchCh)
	Start(w.LogWatch)

	go w.handleFileChange()

}

const (
	AlgorandLogMessage = "algorand.log.message"
)

func (w *AlgorandLogWatch) handleFileChange() {
	for {
		select {
		case message := <-w.logWatchCh:
			jsonMap := message.(map[string]interface{})

			body, err := json.Marshal(jsonMap)
			if err != nil {
				log.Error("[AlgorandLogWatch] failed to marshal json data: ", err)
				continue
			}

			_ = body

			//metric := publisher.Metric{
			//	Type: AlgorandLogMessage,
			//	Body: publisher.MetricBody{
			//		Timestamp: time.Now().UnixMilli(),
			//		Value:     body,
			//	},
			//}
			//
			//w.Emit(metric)

		case <-w.StopKey:
			return

		}
	}
}
