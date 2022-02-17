package watch

import (
	"agent/api/v1/model"
	"encoding/json"

	. "agent/pkg/watch"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

type AlgorandBlockWatchConf struct {
	Path string
}

type AlgorandBlockWatch struct {
	AlgorandBlockWatchConf
	Watch

	JsonLogWatch Watcher
	logCh        chan interface{}
}

func NewAlgorandBlockWatch(conf AlgorandBlockWatchConf, jsonLogWatch Watcher) *AlgorandBlockWatch {
	w := new(AlgorandBlockWatch)
	w.AlgorandBlockWatchConf = conf
	w.Watch = NewWatch()
	w.JsonLogWatch = jsonLogWatch
	w.logCh = make(chan interface{}, 1)
	return w
}

func (w *AlgorandBlockWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	if w.JsonLogWatch == nil {
		w.JsonLogWatch = NewJsonLogWatch(JsonLogWatchConf{Path: w.Path}, nil)
	}
	w.JsonLogWatch.Subscribe(w.logCh)
	go w.handleLogMessage()

	Start(w.JsonLogWatch)
}

func (w *AlgorandBlockWatch) Stop() {
	w.Watch.Stop()

	w.JsonLogWatch.Stop()
}

func (w *AlgorandBlockWatch) handleLogMessage() {
	for {
		select {
		case message := <-w.logCh:
			jsonMessage := message.(map[string]interface{})

			if val, ok := jsonMessage["Type"]; ok && val == "RoundConcluded" {
				round, ok := jsonMessage["Round"]
				if !ok {
					w.Log.Errorw("corrupt log message: missing field 'Round'", "type", val)
					continue
				}

				newBlockEvent := struct {
					Round uint64
				}{
					Round: round.(uint64),
				}

				newBlockEventJson, err := json.Marshal(newBlockEvent)
				if err != nil {
					w.Log.Errorw("failed to marshal new block event", zap.Error(err))
					return
				}

				protoEvent := &model.Event{Body: newBlockEventJson}

				out, err := proto.Marshal(protoEvent)
				if err != nil {
					w.Log.Errorw("failed to proto.Marshal new block event", zap.Error(err))
					return
				}

				metric := model.Message{
					Name:  "protocol.new_block",
					Type:  model.MessageType_event,
					Body: out,
				}

				w.Emit(metric)
			}

		case <-w.StopKey:
			return

		}
	}
}
