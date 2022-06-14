package emit

import (
	"context"
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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
		Name:      "test-message",
		Type:      model.MessageType_event,
		Timestamp: time.Now().UnixMilli(),
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

	evTime := time.Now()
	ev := &model.Event{Name: "test-event"}
	evb, err := proto.Marshal(ev)
	require.Nil(t, err)

	exp := &model.Message{
		Name:      "test-event",
		Type:      model.MessageType_event,
		Timestamp: evTime.UnixMilli(),
		Body:      evb,
	}

	se := &simpleEmitter{emitch}

	err = Ev(se, evTime, ev)
	require.Nil(t, err)

	select {
	case got := <-retch:
		require.Equal(t, exp, got)
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for emitted message")
	}
}
