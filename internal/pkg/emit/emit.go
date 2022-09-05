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

// NewSimpleEmitter returns an object that solely implements the
// Emitter interface. Used to emit events independent of a watchers.
func NewSimpleEmitter(emitch chan<- interface{}) Emitter {
	return &simpleEmitter{emitch: emitch}
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
