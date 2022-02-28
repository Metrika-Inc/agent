package watch

import (
	"agent/api/v1/model"
	"agent/pkg/timesync"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

func EmitEvent(w WatcherEmit, ev *model.Event) error {
	evBytes, err := proto.Marshal(ev)
	if err != nil {
		return err
	}

	message := model.Message{
		Type:      model.MessageType_event,
		Timestamp: timesync.Now().Unix(),
		Body:      evBytes,
	}

	zap.S().Debug("emitting event: ", ev.Name, string(ev.Values))
	w.Emit(message)

	return nil
}
