package factory

import (
	"agent/internal/pkg/global"
	"agent/pkg/collector"
	"agent/pkg/watch"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

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

func NewWatcherByType(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	switch {
	case conf.Type.IsPrometheus(): // prometheus
		var clr prometheus.Collector
		clr = prometheusCollectorsFactory(conf.Type)
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
