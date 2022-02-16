package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/pkg/timesync"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	timesync.Default.Start()
	// l, _ := zap.NewProduction()
	// zap.ReplaceGlobals(l)
	m.Run()
}

type MockAgentClient struct {
	model.UnimplementedAgentServer
	execute func() (*model.PlatformResponse, error)
}

func (m *MockAgentClient) Transmit(ctx context.Context, in *model.PlatformMessage, opts ...grpc.CallOption) (*model.PlatformResponse, error) {
	return m.execute()
}

func newMockAgentClient(execFunc func() (*model.PlatformResponse, error)) *MockAgentClient {
	return &MockAgentClient{execute: execFunc}
}

// TestPublisher_EagerDrain checks:
// - buffer is drained immediately when it reaches MaxBatchLen (before
//   periodic drain timer kicks in)
// - any drained metric is published to the platform
func TestPublisher_EagerDrain(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)

	conf := TransportConf{
		UUID:           "test-agent-uuid",
		Timeout:        10 * time.Second,
		MaxBatchLen:    n / 2,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishIntv:    5 * time.Second,
		BufferTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)
	pub := NewHTTP(pubCh, conf)

	pub.agentService = newMockAgentClient(func() (*model.PlatformResponse, error) {
		platformCh <- nil
		return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
	})

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.Message{
				Timestamp: time.Now().UnixMilli(),
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				AltMetric: body,
			}
			pubCh <- m
			if i == n/2 {
				wg.Done()
			}
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, 0, pub.buffer.Len())

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}
}

// TestPublisher_EagerDrainRegression checks:
// - the buffer is not drained by the publisher if less than MaxBatchLen
//   items are bufferred.
// - buffer drains when periodic draining kicks in
// - any drained metric is published to the platform
func TestPublisher_EagerDrainRegression(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCh <- nil
		w.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := TransportConf{
		UUID:           "test-agent-uuid",
		Timeout:        10 * time.Second,
		MaxBatchLen:    10000,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishIntv:    500 * time.Millisecond,
		BufferTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)

	pub.agentService = newMockAgentClient(func() (*model.PlatformResponse, error) {
		platformCh <- nil
		return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
	})

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.Message{
				Timestamp: time.Now().UnixMilli(),
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				AltMetric: body,
			}
			pubCh <- m
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	<-time.After(conf.PublishIntv)
	require.Equal(t, 0, pub.buffer.Len())
}

// TestPublisher_Error checks:
// - it buffers metrics if platform is unavailable
// - it drains the buffer when platform recovers (healthyAfter)
// - any drained metric is published to the platform
func TestPublisher_Error(t *testing.T) {
	n := 10
	healthyAfter := 500 * time.Millisecond
	st := time.Now()
	platformCh := make(chan interface{}, n)
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if time.Since(st) < healthyAfter {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - Something bad happened!"))
		} else {
			w.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
		}
		platformCh <- nil
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := TransportConf{
		URL:            ts.URL,
		UUID:           "test-agent-uuid",
		Timeout:        10 * time.Second,
		MaxBatchLen:    10,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishIntv:    healthyAfter,
		BufferTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)

	pub.agentService = newMockAgentClient(func() (*model.PlatformResponse, error) {
		defer func() {
			platformCh <- nil
		}()
		if time.Since(st) < healthyAfter {
			return nil, errors.New("foo")
		}
		return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
	})

	wg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.Message{
				Timestamp: time.Now().UnixMilli(),
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				AltMetric: body,
			}
			pubCh <- m
		}
	}()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	<-time.After(conf.PublishIntv)
	require.Equal(t, 0, pub.buffer.Len())

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}
}

// TestPublisher_Stop checks:
// - any buffered metric is published to the platform before the publisher stops
func TestPublisher_Stop(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)

	conf := TransportConf{
		UUID:           "test-agent-uuid",
		Timeout:        10 * time.Second,
		MaxBatchLen:    100,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishIntv:    5 * time.Second,
		BufferTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)

	pub.agentService = newMockAgentClient(func() (*model.PlatformResponse, error) {
		platformCh <- nil
		return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
	})

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.Message{
				Timestamp: time.Now().UnixMilli(),
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				AltMetric: body,
			}
			pubCh <- m
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	pub.Stop()
	pubWg.Wait()

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	require.Equal(t, 0, pub.buffer.Len())
}
