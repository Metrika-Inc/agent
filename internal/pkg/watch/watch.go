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
	"agent/internal/pkg/buf"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var PrometheusWatchPrefix = "prometheus"

type WatchType string

func (w WatchType) IsPrometheus() bool {
	return strings.HasPrefix(string(w), PrometheusWatchPrefix)
}

type Watcher interface {
	StartUnsafe()
	Stop()
	Wait()

	Subscribe(chan<- interface{})

	once() *sync.Once
}

func Start(watcher Watcher) {
	watcher.once().Do(watcher.StartUnsafe)
}

type Watch struct {
	Running bool

	StopKey chan bool
	wg      *sync.WaitGroup

	startOnce *sync.Once
	listeners []chan<- interface{}
	Log       *zap.SugaredLogger
}

func NewWatch() Watch {
	return Watch{
		Running:   false,
		StopKey:   make(chan bool, 1),
		startOnce: &sync.Once{},
		Log:       zap.S(),
		wg:        &sync.WaitGroup{},
	}
}

func (w *Watch) StartUnsafe() {
	w.Running = true
}

func (w *Watch) Wait() {
	w.wg.Wait()
}

func (w *Watch) Stop() {
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

func (w *Watch) Subscribe(handler chan<- interface{}) {
	w.listeners = append(w.listeners, handler)
}

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
