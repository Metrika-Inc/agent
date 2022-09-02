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
	"context"
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
)

func TestSimpleEmitter(t *testing.T) {
	emitch := make(chan interface{}, 1)
	retch := make(chan interface{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case msg, ok := <-emitch:
				if !ok {
					return
				}
				retch <- msg
			case <-ctx.Done():
				return
			}
		}
	}()

	exp := &model.Message{
		Name: "test-message",
	}

	se := simpleEmitter{emitch}
	se.Emit(exp)

	select {
	case got := <-retch:
		require.Equal(t, exp, got)
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for emitted message")
	}
}

func TestEv(t *testing.T) {
	emitch := make(chan interface{}, 1)
	retch := make(chan interface{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case msg, ok := <-emitch:
				if !ok {
					return
				}
				retch <- msg
			case <-ctx.Done():
				return
			}
		}
	}()

	ev := &model.Event{Name: "test-event", Timestamp: time.Now().UnixMilli()}
	exp := &model.Message{
		Name:  "test-event",
		Value: &model.Message_Event{Event: ev},
	}

	se := &simpleEmitter{emitch}

	err := Ev(se, ev)
	require.Nil(t, err)

	select {
	case got := <-retch:
		require.Equal(t, exp, got)
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for emitted message")
	}
}
