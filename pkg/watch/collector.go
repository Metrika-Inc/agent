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
	"encoding/json"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
)

const namespace = "node"

type CollectorWatchConf struct {
	Type      WatchType
	Collector prometheus.Collector
	Gatherer  prometheus.Gatherer
	Interval  time.Duration
}

type CollectorWatch struct {
	CollectorWatchConf
	Watch

	wg              *sync.WaitGroup
	ch              chan prometheus.Metric
	ch1             chan []*dto.MetricFamily
	stopCollectorCh chan bool
}

func NewCollectorWatch(conf CollectorWatchConf) *CollectorWatch {
	w := new(CollectorWatch)
	w.CollectorWatchConf = conf
	w.Watch = NewWatch()

	w.wg = new(sync.WaitGroup)
	w.ch = make(chan prometheus.Metric, 1000)
	w.ch1 = make(chan []*dto.MetricFamily, 1000)
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
				metricFamilies, err := c.Gatherer.Gather()
				if err != nil {
					log.Errorf("[%T] gather error: %v", c.Collector, err)

					continue
				}
				c.ch1 <- metricFamilies
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
		case metricFams := <-c.ch1:
			for _, metricFam := range metricFams {
				body, err := json.Marshal(metricFam)
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

			}
		case <-c.StopKey:
			c.stopCollectorCh <- true
		}
	}
}
