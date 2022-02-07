package global

import (
	"agent/pkg/watch"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	WatcherRegistry    WatchersRegisterer
	PrometheusRegistry prometheus.Registerer
	PrometheusGatherer prometheus.Gatherer

	// Modified at runtime
	Version = "v0.0.0"
	CommitHash = ""
	Protocol = "development"
)

type WatchersRegisterer interface {
	Register(w ...watch.Watcher) error
	Start(ch chan<- interface{}) error
	Stop()
}

type DefaultWatcherRegistrar struct {
	watchers []watch.Watcher
}

func (r *DefaultWatcherRegistrar) Register(w ...watch.Watcher) error {
	r.watchers = append(r.watchers, w...)

	return nil
}

func (r *DefaultWatcherRegistrar) Start(ch chan<- interface{}) error {
	for _, w := range r.watchers {
		w.Subscribe(ch)
		watch.Start(w)
	}

	return nil
}

func (r *DefaultWatcherRegistrar) Stop() {
	for _, w := range r.watchers {
		w.Stop()
	}
}

func init() {
	defaultWatcherRegistrar := new(DefaultWatcherRegistrar)
	defaultWatcherRegistrar.watchers = []watch.Watcher{}
	WatcherRegistry = defaultWatcherRegistrar

	PrometheusRegistry = prometheus.NewPedanticRegistry()
	PrometheusGatherer = PrometheusRegistry.(prometheus.Gatherer)
}
