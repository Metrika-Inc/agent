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
	case conf.Type.IsPEF():
		PEFEndpoints := global.NodeProtocol.PEFEndpoints()
		for _, ep := range PEFEndpoints {
			if watch.PEFWatchPrefix+"."+ep.Name == string(conf.Type) {
				httpConf := watch.HttpGetWatchConf{
					Interval: conf.SamplingInterval,
					Url:      ep.URL,
					Timeout:  time.Second,
				}
				httpWatch := watch.NewHttpGetWatch(httpConf)
				filter := &openmetrics.PEFFilter{ToMatch: ep.Filters}
				pefConf := watch.PEFWatchConf{Filter: filter}
				w = watch.NewPEFWatch(pefConf, httpWatch)
			}
			if w == nil {
				zap.S().Fatalw("unknown PEF collector specified in config",
					"collector_name", conf.Type, "protocol", global.Protocol)
			}
		}
	default:
		zap.S().Fatalw("specified collector type not found", "collector", conf.Type)
	}

	return w
}
