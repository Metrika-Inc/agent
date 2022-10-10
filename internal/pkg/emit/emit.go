// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package emit

import (
	"agent/api/v1/model"

	"go.uber.org/zap"
)

// Emitter interface for emitting events to a channel
type Emitter interface {
	Emit(message interface{})
}

type simpleEmitter struct {
	emitch chan<- interface{}
}

// Emit emits messages to the configured channel.
func (s *simpleEmitter) Emit(message interface{}) {
	if s.emitch == nil {
		zap.S().Error("emit channel not configured")

		return
	}

	s.emitch <- message
}

type multiEmitter struct {
	emitChs []chan<- interface{}
}

// Emit emits messages to the configured list of channels.
func (m *multiEmitter) Emit(message interface{}) {
	if len(m.emitChs) == 0 {
		zap.S().Error("emit channels empty")
	}

	for i := range m.emitChs {
		if m.emitChs[i] == nil {
			zap.S().Error("channel misconfigured", "index", i)
			continue
		}
		select {
		case m.emitChs[i] <- message:
		default:
			zap.S().Warnw("handler channel block an event, discarding it", "handler_no", i)
		}

	}
}

// NewSimpleEmitter returns an object that solely implements the
// Emitter interface. Used to emit events independent of a watchers.
func NewSimpleEmitter(emitch chan<- interface{}) Emitter {
	return &simpleEmitter{emitch: emitch}
}

// NewMultiEmitter returns an object that implements the Emitter interface.
// Use to inform all the exporters (event subscribers) about the ocurring events.
func NewMultiEmitter(emitChs []chan<- interface{}) Emitter {
	return &multiEmitter{emitChs: emitChs}
}

// Ev builds a new event message compatible for publishing and pushes
// it to the publisher by executing the watcher's Emit() function.
func Ev(w Emitter, ev *model.Event) error {
	message := model.Message{
		Name:  ev.GetName(),
		Value: &model.Message_Event{Event: ev},
	}

	zap.S().Debugw("emitting event", "event", ev.Name, "map", ev.Values.AsMap())

	w.Emit(&message)

	return nil
}
