package emit

import (
	"agent/api/v1/model"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
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

// NewSimpleEmitter returns an object that solely implements the
// Emitter interface. Used to emit events independent of a watchers.
func NewSimpleEmitter(emitch chan<- interface{}) *SimpleEmitter {
	return &SimpleEmitter{Emitch: emitch}
}

// Ev builds a new event message compatible for publishing and pushes
// it to the publisher by executing the watcher's Emit() function.
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
