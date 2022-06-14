package factory

import (
	"agent/internal/pkg/global"
	"agent/internal/pkg/watch"
	"agent/pkg/collector"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type WatchersRegisterer interface {
	Register(w ...watch.Watcher) error
	Start(ch ...chan<- interface{}) error
	Stop()
	Wait()
}

var WatcherRegistry WatchersRegisterer

func init() {
	defaultWatcherRegistrar := new(DefaultWatcherRegistrar)

	defaultWatcherRegistrar.watchers = []watch.Watcher{}
	WatcherRegistry = defaultWatcherRegistrar
}

func NewWatcherByType(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	wt := watch.WatchType(conf.Type)
	switch {
	case wt.IsPrometheus(): // prometheus
		var clr prometheus.Collector
		clr = prometheusCollectorsFactory(wt)
		registry := prometheus.NewPedanticRegistry()
		w = watch.NewCollectorWatch(watch.CollectorWatchConf{
			Type:      watch.WatchType(conf.Type),
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

type DefaultWatcherRegistrar struct {
	watchers []watch.Watcher
}

func (r *DefaultWatcherRegistrar) Register(w ...watch.Watcher) error {
	r.watchers = append(r.watchers, w...)

	return nil
}

func (r *DefaultWatcherRegistrar) Start(ch ...chan<- interface{}) error {
	for _, w := range r.watchers {
		for _, c := range ch {
			w.Subscribe(c)
		}

		go func(w watch.Watcher) {
			watch.Start(w)
		}(w)
	}

	return nil
}

func (r *DefaultWatcherRegistrar) Stop() {
	for _, w := range r.watchers {
		w.Stop()
	}
}

func (r *DefaultWatcherRegistrar) Wait() {
	for _, w := range r.watchers {
		w.Wait()
	}
}

// prometheusCollectorsFactory creates watchers backed by pkg/collector.
func prometheusCollectorsFactory(t watch.WatchType) prometheus.Collector {
	clrFunc, ok := collector.CollectorsFactory[string(t)]
	if !ok {
		zap.S().Fatalw("specified prometheus collector constructor not found", "collector", t)
	}

	clr, err := clrFunc()
	if err != nil {
		zap.S().Fatalw("collector constructor error", zap.Error(err))
	}

	return clr
}
