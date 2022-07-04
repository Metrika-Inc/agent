package emit

import (
	"time"

	"agent/api/v1/model"

	"go.uber.org/zap"
)

type Emitter interface {
	Emit(message interface{})
}

type simpleEmitter struct {
	emitch chan<- interface{}
}

func (s *simpleEmitter) Emit(message interface{}) {
	if s.emitch == nil {
		zap.S().Error("emit channel not configured")

		return
	}

	s.emitch <- message
}

// NewSimpleEmitter returns an object that solely implements the
// Emitter interface. Used to emit events independent of a watchers.
func NewSimpleEmitter(emitch chan<- interface{}) *simpleEmitter {
	return &simpleEmitter{emitch: emitch}
}

// Ev builds a new event message compatible for publishing and pushes
// it to the publisher by executing the watcher's Emit() function.
func Ev(w Emitter, t time.Time, ev *model.Event) error {
	message := model.Message{
		Name:      ev.GetName(),
		Type:      model.MessageType_event,
		Timestamp: t.UnixMilli(),
		Value:     &model.Message_Event{Event: ev},
	}

	zap.S().Debugw("emitting event", "event", ev.Name, "map", ev.Values.AsMap())

	w.Emit(&message)

	return nil
}
