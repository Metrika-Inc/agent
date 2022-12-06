package watch

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndStart_Success(t *testing.T) {
	w := NewWatch()
	registry := &Registry{
		watch:      []*WatcherInstance{},
		Mutex:      &sync.Mutex{},
		watcherMap: make(map[Watcher]struct{}),
	}

	err := registry.RegisterAndStart(&w, nil)
	require.NoError(t, err)
	require.Len(t, registry.watch, 1)

	<-time.After(50 * time.Millisecond)
	w.Lock()
	require.True(t, w.Running)
	w.Unlock()
}

func TestRegistry_Register_MultipleCalls(t *testing.T) {
	w := NewWatch()
	w.listeners = make([]chan<- interface{}, 0)
	w.listeners = append(w.listeners, make(chan<- interface{}))
	registry := &Registry{
		watch:      []*WatcherInstance{},
		Mutex:      &sync.Mutex{},
		watcherMap: make(map[Watcher]struct{}),
	}

	var err error
	err = registry.Register(&w)
	require.NoError(t, err)
	err = registry.Register(&w)
	require.NoError(t, err)

	// expect idempotency - only 1 watcher should exist in registry
	require.Len(t, registry.watch, 1)
}

func TestRegistry_Register_Regression(t *testing.T) {
	// ensure that the registration continues to work
	// both for different instances of same type of watchers
	// as well as different watcher types
	sameConf := HTTPWatchConf{}
	w1 := NewHTTPWatch(sameConf)
	w2 := NewCollectorWatch(CollectorWatchConf{})
	w3 := NewDockerLogWatch(DockerLogWatchConf{})
	w4 := NewHTTPWatch(sameConf)
	w1.Subscribe(make(chan<- interface{}))
	w2.Subscribe(make(chan<- interface{}))
	w3.Subscribe(make(chan<- interface{}))
	w4.Subscribe(make(chan<- interface{}))

	registry := &Registry{
		watch:      []*WatcherInstance{},
		Mutex:      &sync.Mutex{},
		watcherMap: make(map[Watcher]struct{}),
	}

	err := registry.Register(w1, w2, w3, w4)
	require.NoError(t, err)
	require.Len(t, registry.watch, 4)

	err = registry.Register(w4, w3, w2, w1)
	require.NoError(t, err)
	require.Len(t, registry.watch, 4)
}
