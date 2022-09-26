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

package publisher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/buf"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/global"
	"agent/internal/pkg/transport"
	"agent/pkg/timesync"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestMain(m *testing.M) {
	global.BlockchainNode = &discover.MockBlockchain{}
	timesync.Default.Start(nil)
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

type MockAgentClientWithCtx struct {
	model.UnimplementedAgentServer
	execute func(context.Context) (*model.PlatformResponse, error)

	ctx context.Context
}

func (m *MockAgentClientWithCtx) Transmit(ctx context.Context, in *model.PlatformMessage, opts ...grpc.CallOption) (*model.PlatformResponse, error) {
	return m.execute(ctx)
}

func newMockAgentClientWithCtx(execFunc func(ctx context.Context) (*model.PlatformResponse, error)) *MockAgentClientWithCtx {
	return &MockAgentClientWithCtx{execute: execFunc}
}

// TestPublisher_EagerDrain checks:
//   - buffer is drained immediately when it reaches MaxBatchLen (before
//     periodic drain timer kicks in)
//   - any drained metric is published to the platform
func TestPublisher_EagerDrain(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)

	conf := testConf{
		UUID:            "test-agent-uuid",
		APIKey:          "test-api-key",
		TransmitTimeout: 10 * time.Second,
		MaxBatchLen:     n / 2,
		MaxBufferBytes:  uint(50 * 1024 * 1024),
		PublishIntv:     5 * time.Second,
		BufferTTL:       time.Duration(0),
	}

	mocksvc := newMockAgentClient(
		func() (*model.PlatformResponse, error) {
			platformCh <- nil
			return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
		})

	transportConf := transport.PlatformGRPCConf{
		URL:             "mockurl",
		UUID:            conf.UUID,
		APIKey:          conf.APIKey,
		TransmitTimeout: conf.TransmitTimeout,
		AgentService:    mocksvc,
	}

	tr, err := transport.NewPlatformGRPC(transportConf)
	require.Nil(t, err)
	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         conf.MaxBatchLen,
		BufDrainFreq:        conf.PublishIntv,
		OnBufRemoveCallback: tr.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(conf.BufferTTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	pub := NewPublisher(Config{}, bufCtrl)

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < n; i++ {
			m := &model.Message{
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				Value:     &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
			}
			pub.HandleMessage(context.Background(), m)
			if i == n/2 {
				wg.Done()
			}
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, 0, pub.bufCtrl.B.Len())

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
//   - the buffer is not drained by the publisher if less than MaxBatchLen
//     items are bufferred.
//   - buffer drains when periodic draining kicks in
//   - any drained metric is published to the platform
func TestPublisher_EagerDrainRegression(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCh <- nil
		w.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := testConf{
		UUID:            "test-agent-uuid",
		APIKey:          "test-api-key",
		TransmitTimeout: 10 * time.Second,
		MaxBatchLen:     10000,
		MaxBufferBytes:  uint(50 * 1024 * 1024),
		PublishIntv:     500 * time.Millisecond,
		BufferTTL:       time.Duration(0),
	}

	mocksvc := newMockAgentClient(
		func() (*model.PlatformResponse, error) {
			platformCh <- nil
			return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
		})

	transportConf := transport.PlatformGRPCConf{
		URL:             "mockurl",
		UUID:            conf.UUID,
		APIKey:          conf.APIKey,
		TransmitTimeout: conf.TransmitTimeout,
		AgentService:    mocksvc,
	}

	tr, err := transport.NewPlatformGRPC(transportConf)
	require.Nil(t, err)

	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         conf.MaxBatchLen,
		BufDrainFreq:        conf.PublishIntv,
		OnBufRemoveCallback: tr.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(conf.BufferTTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	pub := NewPublisher(Config{}, bufCtrl)

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < n; i++ {
			m := &model.Message{
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				Value:     &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
			}
			pub.HandleMessage(context.Background(), m)
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n, pub.bufCtrl.B.Len()) // +1 for the very first agent.up event

	<-time.After(conf.PublishIntv)
	require.Equal(t, 0, pub.bufCtrl.B.Len())
}

// TestPublisher_Error checks:
// - it buffers metrics if platform is unavailable
// - it drains the buffer when platform recovers (healthyAfter)
// - any drained metric is published to the platform
func TestPublisher_Error(t *testing.T) {
	n := 10

	healthyAfter := 300 * time.Millisecond
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

	conf := testConf{
		URL:             ts.URL,
		UUID:            "test-agent-uuid",
		APIKey:          "test-api-key",
		TransmitTimeout: 10 * time.Second,
		MaxBatchLen:     10 + 1, // + 1 to compensate for agent.net.error event
		MaxBufferBytes:  uint(50 * 1024 * 1024),
		PublishIntv:     healthyAfter,
		BufferTTL:       time.Duration(0),
	}

	mocksvc := newMockAgentClient(
		func() (*model.PlatformResponse, error) {
			defer func() {
				platformCh <- nil
			}()
			if time.Since(st) < healthyAfter {
				return nil, errors.New("foo")
			}
			return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
		})

	transportConf := transport.PlatformGRPCConf{
		URL:             "mockurl",
		UUID:            conf.UUID,
		APIKey:          conf.APIKey,
		TransmitTimeout: conf.TransmitTimeout,
		AgentService:    mocksvc,
	}

	tr, err := transport.NewPlatformGRPC(transportConf)
	require.Nil(t, err)
	tr.GrpcErrHandler = func() error {
		tr.AgentService = nil
		return nil
	}

	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         conf.MaxBatchLen,
		BufDrainFreq:        conf.PublishIntv,
		OnBufRemoveCallback: tr.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(conf.BufferTTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	pub := NewPublisher(Config{}, bufCtrl)

	wg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			m := &model.Message{
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				Value:     &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
			}
			pub.HandleMessage(context.Background(), m)
		}
	}()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n+2, pub.bufCtrl.B.Len()) // + 1 compensate for agent.net.error event

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	<-time.After(conf.PublishIntv)
	require.Equal(t, 1, pub.bufCtrl.B.Len())
}

type testConf struct {
	URL             string
	APIKey          string
	UUID            string
	TransmitTimeout time.Duration
	MaxBatchLen     int
	MaxBufferBytes  uint
	PublishIntv     time.Duration
	BufferTTL       time.Duration
}

// TestPublisher_Stop checks:
// - any buffered metric is published to the platform before the publisher stops
func TestPublisher_Stop(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)

	conf := testConf{
		UUID:            "test-agent-uuid",
		APIKey:          "test-api-key",
		TransmitTimeout: 10 * time.Second,
		MaxBatchLen:     100,
		MaxBufferBytes:  uint(50 * 1024 * 1024),
		PublishIntv:     5 * time.Second,
		BufferTTL:       time.Duration(0),
	}

	mocksvc := newMockAgentClient(
		func() (*model.PlatformResponse, error) {
			platformCh <- nil
			return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
		})

	transportConf := transport.PlatformGRPCConf{
		URL:             "mockurl",
		UUID:            "test-agent-uuid",
		APIKey:          conf.APIKey,
		TransmitTimeout: conf.TransmitTimeout,
		AgentService:    mocksvc,
	}

	tr, err := transport.NewPlatformGRPC(transportConf)
	require.Nil(t, err)

	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         conf.MaxBatchLen,
		BufDrainFreq:        conf.PublishIntv,
		OnBufRemoveCallback: tr.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(conf.BufferTTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	pub := NewPublisher(Config{}, bufCtrl)

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < n; i++ {
			m := &model.Message{
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				Value:     &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
			}
			pub.HandleMessage(context.Background(), m)
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, n, pub.bufCtrl.B.Len())

	pub.Stop()
	pubWg.Wait()

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	require.Equal(t, 0, pub.bufCtrl.B.Len())
}

// TestPublisher_GRPCMetadata checks:
// - PlatformMessage is accompanied by GRPC metadata
func TestPublisher_GRPCMetadata(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)

	conf := testConf{
		UUID:            "test-agent-uuid",
		APIKey:          "test-api-key",
		TransmitTimeout: 10 * time.Second,
		MaxBatchLen:     n / 2,
		MaxBufferBytes:  uint(50 * 1024 * 1024),
		PublishIntv:     5 * time.Second,
		BufferTTL:       time.Duration(0),
	}

	mocksvc := newMockAgentClientWithCtx(
		func(ctx context.Context) (*model.PlatformResponse, error) {
			md, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)

			platformCh <- md

			return &model.PlatformResponse{Timestamp: time.Now().UnixNano()}, nil
		})

	transportConf := transport.PlatformGRPCConf{
		URL:             "mockurl",
		UUID:            conf.UUID,
		APIKey:          conf.APIKey,
		TransmitTimeout: conf.TransmitTimeout,
		AgentService:    mocksvc,
	}

	tr, err := transport.NewPlatformGRPC(transportConf)
	require.Nil(t, err)

	bufCtrlConf := buf.ControllerConf{
		BufLenLimit:         conf.MaxBatchLen,
		BufDrainFreq:        conf.PublishIntv,
		OnBufRemoveCallback: tr.PublishFunc,
	}

	buffer := buf.NewPriorityBuffer(conf.BufferTTL)
	bufCtrl := buf.NewController(bufCtrlConf, buffer)

	pub := NewPublisher(Config{}, bufCtrl)

	pubWg := new(sync.WaitGroup)
	timesync.Listen()
	pub.Start(pubWg)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			m := &model.Message{
				Name:      "test-metric",
				NodeState: model.NodeState_up,
				Value:     &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
			}
			pub.HandleMessage(context.Background(), m)
		}
	}()
	wg.Wait()

	<-time.After(200 * time.Millisecond)
	require.Equal(t, 0, pub.bufCtrl.B.Len())

	select {
	case got := <-platformCh:
		require.IsType(t, metadata.MD{}, got)
		md := got.(metadata.MD)

		require.Equal(t, conf.UUID, md.Get(transport.AgentUUIDHeaderName)[0])
		require.Equal(t, conf.APIKey, md.Get(transport.AgentAPIKeyHeaderName)[0])
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	select {
	case got := <-platformCh:
		require.IsType(t, metadata.MD{}, got)
		md := got.(metadata.MD)

		require.Equal(t, conf.UUID, md.Get(transport.AgentUUIDHeaderName)[0])
		require.Equal(t, conf.APIKey, md.Get(transport.AgentAPIKeyHeaderName)[0])
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}
}
