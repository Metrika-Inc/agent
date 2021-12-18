// Copyright 2015 The Prometheus Authors
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

// Package collector includes all individual collectors to gather and export system metrics.
package watch

import (
	"agent/api/v1/model"
	"agent/pkg/collector"
	"encoding/json"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
)

const namespace = "node"

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"node_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"node_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

func execute(name string, c collector.Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if collector.IsNoDataError(err) {
			log.Debug("msg", "collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			log.Error("msg", "collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
		success = 0
	} else {
		log.Debug("msg", "collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

type CollectorWatchConf struct {
	Type      WatchType
	Collector collector.Collector
	Interval  time.Duration
}

type CollectorWatch struct {
	CollectorWatchConf
	Watch

	wg              *sync.WaitGroup
	ch              chan prometheus.Metric
	stopCollectorCh chan bool
}

func NewCollectorWatch(conf CollectorWatchConf) *CollectorWatch {
	w := new(CollectorWatch)
	w.CollectorWatchConf = conf
	w.Watch = NewWatch()

	w.wg = new(sync.WaitGroup)
	w.ch = make(chan prometheus.Metric, 1000)
	w.stopCollectorCh = make(chan bool, 1)

	return w
}

func (c *CollectorWatch) StartUnsafe() {
	c.Watch.StartUnsafe()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-time.After(c.Interval):
				err := c.Collector.Update(c.ch)
				if err != nil {
					log.Errorf("[%T] collector update error: %v", c.Collector, err)

					continue
				}
			case <-c.stopCollectorCh:
			}
		}
	}()

	// Listen to events
	go c.handlePrometheusMetric()
}

func (c *CollectorWatch) handlePrometheusMetric() {
	for {
		select {
		case metric := <-c.ch:
			dtoMetric := dto.Metric{}
			err := metric.Write(&dtoMetric)
			if err != nil {
				log.Errorf("[%T] cannot write dto.Metric: %v", c.Collector, err)

				continue
			}

			body, err := json.Marshal(dtoMetric)
			if err != nil {
				log.Errorf("[%T] cannot marshal dto.Metric: %v", c.Collector, err)

				continue
			}

			// Create & emit the metric
			metricInternal := model.MetricPlatform{
				Type:      string(c.Type),
				Timestamp: time.Now().UTC().UnixMilli(),
				Body:      body,
			}

			c.Emit(metricInternal)
		case <-c.StopKey:
			c.stopCollectorCh <- true
		}
	}
}
