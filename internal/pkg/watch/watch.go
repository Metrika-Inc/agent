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
	"encoding/json"
	"sync"

	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"go.uber.org/zap"
)

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

	startOnce  *sync.Once
	listeners  []chan<- interface{}
	Log        *zap.SugaredLogger
	blockchain global.Chain
	*sync.Mutex
}

// NewWatch base watch constructor
func NewWatch() Watch {
	return Watch{
		Running:    false,
		StopKey:    make(chan bool, 1),
		startOnce:  &sync.Once{},
		Log:        zap.S(),
		wg:         &sync.WaitGroup{},
		Mutex:      &sync.Mutex{},
		blockchain: global.BlockchainNode(),
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
			global.MetricsDropCnt.WithLabelValues("channel_blocked").Inc()
		}
	}
}

func (w *Watch) parseJSON(body []byte) (map[string]interface{}, error) {
	var jsonResult map[string]interface{}
	err := json.Unmarshal(body, &jsonResult)
	if err != nil {
		return nil, err
	}

	return jsonResult, nil
}

func (w *Watch) emitNodeLogEvents(evs map[string]model.FromContext, body map[string]interface{}) {
	// search for & emit events
	for _, event := range evs {
		ev, err := event.New(body, timesync.Now())
		if err != nil {
			zap.S().Warnf("event construction error %v", err)

			continue
		}

		if ev == nil {
			// nothing to do
			continue
		}

		message := model.Message{
			Name:  ev.Name,
			Value: &model.Message_Event{Event: ev},
		}

		w.Log.Debugw("emitting event", "event", ev.Name, "time", ev.Values.AsMap()["time"])

		w.Emit(&message)
	}
}

func (w *Watch) emitAgentNodeEvent(name string) {
	ctx := map[string]interface{}{}

	nodeID := w.blockchain.NodeID()
	if nodeID != "" {
		ctx[model.NodeIDKey] = nodeID
	}

	nodeType := w.blockchain.NodeRole()
	if nodeType != "" {
		ctx[model.NodeTypeKey] = nodeType
	}

	nodeVersion := w.blockchain.NodeVersion()
	if nodeVersion != "" {
		ctx[model.NodeVersionKey] = nodeVersion
	}

	network := w.blockchain.Network()
	if network != "" {
		ctx[model.NetworkKey] = network
	}

	ev, err := model.NewWithCtx(ctx, name, timesync.Now())
	if err != nil {
		w.Log.Errorw("error creating event: ", zap.Error(err))

		return
	}

	message := model.Message{
		Name:  name,
		Value: &model.Message_Event{Event: ev},
	}

	w.Log.Debugw("emitting event", "event", ev.Name, "map", ev.Values.AsMap())

	w.Emit(&message)
}
