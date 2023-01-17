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
	"net/url"

	"agent/internal/pkg/global"
	"agent/internal/pkg/watch"
	"agent/pkg/collector"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// NewWatcherByType builds and registers a new watcher to the
// default watcher registry.
func NewWatcherByType(conf global.WatchConfig) (watch.Watcher, error) {
	var w watch.Watcher
	wt := global.WatchType(conf.Type)
	switch {
	case wt.IsPrometheus(): // prometheus
		var clr prometheus.Collector
		clr = prometheusCollectorsFactory(collector.Name(wt))
		registry := prometheus.NewPedanticRegistry()
		w = watch.NewCollectorWatch(watch.CollectorWatchConf{
			Type:      global.WatchType(conf.Type),
			Collector: clr,
			Gatherer:  registry,
			Interval:  conf.SamplingInterval,
		})
		registry.MustRegister(clr)
	case wt.IsInflux(): // influx
		influxdbURL, err := url.Parse(conf.UpstreamURL)
		if err != nil {
			return nil, err
		}

		w = watch.NewInfluxExporterWatch(watch.InfluxExporterWatchConf{
			Type:              global.WatchType(conf.Type),
			UpstreamURL:       influxdbURL,
			ListenAddr:        conf.ListenAddr,
			PlatformEnabled:   *global.AgentConf.Platform.Enabled,
			ExporterActivated: conf.ExporterActivated,
		})
	default:
		zap.S().Fatalw("specified collector type not found", "collector", conf.Type)
	}

	return w, nil
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
