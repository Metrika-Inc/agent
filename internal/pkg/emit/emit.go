package emit

import (
	"agent/api/v1/model"
	"time"

	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

type Emitter interface {
	Emit(message interface{})
}

type SimpleEmitter struct {
	Emitch chan<- interface{}
}

func (s *SimpleEmitter) Emit(message interface{}) {
	if s.Emitch == nil {
		zap.S().Error("emit channel not configured")

		return
	}

	s.Emitch <- message
}

func Ev(w Emitter, t time.Time, ev *model.Event) error {
	evBytes, err := proto.Marshal(ev)
	if err != nil {
		return err
	}

	message := model.Message{
		Name:      ev.GetName(),
		Type:      model.MessageType_event,
		Timestamp: t.UnixMilli(),
		Body:      evBytes,
	}

	zap.S().Debug("emitting event: ", ev.Name, ", ", ev.Values.AsMap())

	w.Emit(message)

	return nil
}
