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

package factory

import (
	"agent/internal/pkg/global"
	"agent/internal/pkg/watch"
	"agent/pkg/collector"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WatchersRegisterer is an interface for enabling agent watchers.
type WatchersRegisterer interface {
	Register(w ...watch.Watcher) error
	Start(ch ...chan<- interface{}) error
	Stop()
	Wait()
}

// DefaultWatchRegistry is the default watcher registry used by the agent.
var DefaultWatchRegistry WatchersRegisterer

func init() {
	defaultWatchRegistrar := new(WatchRegistry)

	defaultWatchRegistrar.watch = []watch.Watcher{}
	DefaultWatchRegistry = defaultWatchRegistrar
}

// NewWatcherByType builds and registers a new watcher to the
// default watcher registry.
func NewWatcherByType(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	wt := watch.Type(conf.Type)
	switch {
	case wt.IsPrometheus(): // prometheus
		var clr prometheus.Collector
		clr = prometheusCollectorsFactory(collector.Name(wt))
		registry := prometheus.NewPedanticRegistry()
		w = watch.NewCollectorWatch(watch.CollectorWatchConf{
			Type:      watch.Type(conf.Type),
			Collector: clr,
			Gatherer:  registry,
			Interval:  conf.SamplingInterval,
		})
		registry.MustRegister(clr)
	default:
		zap.S().Fatalw("specified collector type not found", "collector", conf.Type)
	}

	return w
}

// WatchRegistry type
type WatchRegistry struct {
	watch []watch.Watcher
}

// Register registrers one or more watchers
func (r *WatchRegistry) Register(w ...watch.Watcher) error {
	r.watch = append(r.watch, w...)

	return nil
}

// Start starts a watch by subscribing to one or more channels
// for emitting collected data.
func (r *WatchRegistry) Start(ch ...chan<- interface{}) error {
	for _, w := range r.watch {
		for _, c := range ch {
			w.Subscribe(c)
		}

		go func(w watch.Watcher) {
			watch.Start(w)
		}(w)
	}

	return nil
}

// Stop stops all registered watches
func (r *WatchRegistry) Stop() {
	for _, w := range r.watch {
		w.Stop()
	}
}

// Wait for all registered watches to finish
func (r *WatchRegistry) Wait() {
	for _, w := range r.watch {
		w.Wait()
	}
}

// prometheusCollectorsFactory creates watchers backed by pkg/collector.
func prometheusCollectorsFactory(c collector.Name) prometheus.Collector {
	clrFunc, ok := collector.CollectorsFactory[c]
	if !ok {
		zap.S().Fatalw("specified prometheus collector constructor not found", "collector", c)
	}

	clr, err := clrFunc()
	if err != nil {
		zap.S().Fatalw("collector constructor error", zap.Error(err))
	}

	return clr
}
