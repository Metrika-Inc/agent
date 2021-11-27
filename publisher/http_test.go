package publisher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"
	"github.com/stretchr/testify/require"
)

// TestPublisher_EagerDrain checks:
// - buffer is drained immediately when it reaches MaxBatchLen (before
//   periodic drain timer kicks in)
// - any drained metric is published to the platform
func TestPublisher_EagerDrain(t *testing.T) {
	n := 10
	platformCh := make(chan interface{}, n)
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCh <- nil
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := HTTPConf{
		URL:            ts.URL,
		UUID:           "test-agent-uuid",
		DefaultTimeout: 10 * time.Second,
		MaxBatchLen:    n / 2,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishFreq:    5 * time.Second,
		MetricTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)
	wg := new(sync.WaitGroup)
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.MetricPlatform{
				Timestamp: time.Now().UnixMilli(),
				Type:      "test-metric",
				NodeState: model.NodeStateUp,
				Body:      body,
			}
			pubCh <- m
		}
	}()

	<-time.After(100 * time.Millisecond)
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
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := HTTPConf{
		URL:            ts.URL,
		UUID:           "test-agent-uuid",
		DefaultTimeout: 10 * time.Second,
		MaxBatchLen:    10000,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishFreq:    500 * time.Millisecond,
		MetricTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)
	wg := new(sync.WaitGroup)
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.MetricPlatform{
				Timestamp: time.Now().UnixMilli(),
				Type:      "test-metric",
				NodeState: model.NodeStateUp,
				Body:      body,
			}
			pubCh <- m
		}
	}()

	<-time.After(100 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	<-time.After(conf.PublishFreq)
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
			w.Write([]byte("hello client"))
		}
		platformCh <- nil
	})
	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := HTTPConf{
		URL:            ts.URL,
		UUID:           "test-agent-uuid",
		DefaultTimeout: 10 * time.Second,
		MaxBatchLen:    10,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishFreq:    healthyAfter,
		MetricTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)
	wg := new(sync.WaitGroup)
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.MetricPlatform{
				Timestamp: time.Now().UnixMilli(),
				Type:      "test-metric",
				NodeState: model.NodeStateUp,
				Body:      body,
			}
			pubCh <- m
		}
	}()

	<-time.After(100 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	<-time.After(conf.PublishFreq)
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
	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCh <- nil
	})

	ts := httptest.NewServer(handleFunc)
	defer ts.Close()

	conf := HTTPConf{
		URL:            ts.URL,
		UUID:           "test-agent-uuid",
		DefaultTimeout: 10 * time.Second,
		MaxBatchLen:    100,
		MaxBufferBytes: uint(50 * 1024 * 1024),
		PublishFreq:    5 * time.Second,
		MetricTTL:      time.Duration(0),
	}

	pubCh := make(chan interface{}, n)

	pub := NewHTTP(pubCh, conf)
	wg := new(sync.WaitGroup)
	pub.Start(wg)
	go func() {
		for i := 0; i < n; i++ {
			body, _ := json.Marshal([]byte("foobar"))
			m := model.MetricPlatform{
				Timestamp: time.Now().UnixMilli(),
				Type:      "test-metric",
				NodeState: model.NodeStateUp,
				Body:      body,
			}
			pubCh <- m
		}
	}()
	<-time.After(100 * time.Millisecond)
	require.Equal(t, n, pub.buffer.Len())

	pub.Stop()
	wg.Wait()

	select {
	case <-platformCh:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for platform message")
	}

	require.Equal(t, 0, pub.buffer.Len())
}
