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

package watch

import (
	"strings"
	"sync"

	"agent/internal/pkg/buf"

	"go.uber.org/zap"
)

// PrometheusWatchPrefix prefix used for tagging model.Message by
// all node exporter watches.
var PrometheusWatchPrefix = "prometheus"

// Type used for determining is data originates by
// a node exporter watch.
type Type string

// IsPrometheus returns true if watch collects data from node exporter
func (w Type) IsPrometheus() bool {
	return strings.HasPrefix(string(w), PrometheusWatchPrefix)
}

// Watcher is an interface for implementing metric collection.
type Watcher interface {
	StartUnsafe()
	Stop()
	Wait()

	Subscribe(chan<- interface{})

	once() *sync.Once
}

// Start starts a watcher once.
func Start(watcher Watcher) {
	watcher.once().Do(watcher.StartUnsafe)
}

// Watch is the base Watch implementation used by all implemented
// watchers by the agent and can push data to all watcher's subscribed
// channels.
type Watch struct {
	Running bool

	StopKey chan bool
	wg      *sync.WaitGroup

	startOnce *sync.Once
	listeners []chan<- interface{}
	Log       *zap.SugaredLogger
	*sync.Mutex
}

// NewWatch base watch constructor
func NewWatch() Watch {
	return Watch{
		Running:   false,
		StopKey:   make(chan bool, 1),
		startOnce: &sync.Once{},
		Log:       zap.S(),
		wg:        &sync.WaitGroup{},
		Mutex:     &sync.Mutex{},
	}
}

// StartUnsafe sets watch running state to true
func (w *Watch) StartUnsafe() {
	w.Lock()
	defer w.Unlock()
	w.Running = true
}

// Wait blocks waiting for watch goroutine to finish.
func (w *Watch) Wait() {
	w.wg.Wait()
}

// Stop stops the watch
func (w *Watch) Stop() {
	w.Lock()
	defer w.Unlock()
	if !w.Running {
		return
	}
	w.Running = false

	close(w.StopKey)
}

func (w *Watch) once() *sync.Once {
	return w.startOnce
}

// Subscription mechanism

// Subscribe adds a channel to the subscribed listeners slice.
func (w *Watch) Subscribe(handler chan<- interface{}) {
	w.listeners = append(w.listeners, handler)
}

// Emit sends a message to all subscribed channels (i.e publisher, exporter)
func (w *Watch) Emit(message interface{}) {
	for i, handler := range w.listeners {
		select {
		case handler <- message:
		default:
			zap.S().Warnw("handler channel blocked a metric, discarding it", "handler_no", i)
			buf.MetricsDropCnt.WithLabelValues("channel_blocked").Inc()
		}
	}
}
