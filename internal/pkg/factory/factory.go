package factory

import (
	"agent/internal/pkg/global"
	"agent/pkg/collector"
	"agent/pkg/watch"

	algorandWatch "agent/algorand/pkg/watch"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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
		logrus.Fatalf("[algorand] watcher constructor for type %q not found", conf.Type)
	}

	return w
}

// prometheusCollectorsFactory creates watchers backed by pkg/collector.
func prometheusCollectorsFactory(t watch.WatchType) prometheus.Collector {
	clrFunc, ok := collector.CollectorsFactory[string(t)]
	if !ok {
		logrus.Fatalf("[prometheus] collector constructor for type %q not found", t)
	}

	clr, err := clrFunc()
	if err != nil {
		logrus.Fatalf("[prometheus] collector contstructor error: %v", err)
	}

	return clr
}

func NewWatcherByType(conf global.WatchConfig) watch.Watcher {
	var w watch.Watcher
	var err error
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
	default:
		logrus.Fatalf("collector for type %q not found", conf.Type)
	}

	if err != nil {
		logrus.Fatal(err)
	}

	return w
}
