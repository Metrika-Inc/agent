package factory

import (
	"agent/internal/pkg/global"
	"agent/pkg/collector"
	"agent/pkg/parse/openmetrics"
	"agent/pkg/watch"
	"time"

	algorandWatch "agent/algorand/pkg/watch"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// algorandWatchersFactory creates algorand specific watchers
func algorandWatchersFactory(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	switch conf.Type {
	case watch.AlgorandNodeRestart:
		w = algorandWatch.NewAlgodRestartWatch(algorandWatch.AlgodRestartWatchConf{
			Path: "/var/lib/algorand/algod.pid",
		}, nil)
	default:
		zap.S().Fatalw("specified watcher constructor not found", "watcher", conf.Type)
	}

	return w
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

func NewWatcherByType(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	switch {
	case conf.Type.IsAlgorand(): // algorand
		w = algorandWatchersFactory(conf)
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
	case conf.Type.IsPef():
		url, ok := conf.Params["endpoint"].(string)
		if !ok {
			zap.S().Fatalf("expected string as endpoint parameter, got: '%v'", url)
		}

		filter, ok := conf.Params["filter"].([]string)
		if !ok {
			zap.S().Fatalf("expected list as filter parameter, got: '%v'", filter)
		}
		pefFilter := openmetrics.PEFFilter{ToMatch: filter}

		httpConf := watch.HttpGetWatchConf{
			Interval: conf.SamplingInterval,
			Url:      "http://127.0.0.1:9000/metrics",
			Timeout:  time.Second,
		}
		httpWatch := watch.NewHttpGetWatch(httpConf)

		pefConf := watch.PefWatchConf{Filter: pefFilter}
		w = watch.NewPefWatch(pefConf, httpWatch)
	default:
		zap.S().Fatalw("specified collector type not found", "collector", conf.Type)
	}

	return w
}
