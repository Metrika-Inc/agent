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
