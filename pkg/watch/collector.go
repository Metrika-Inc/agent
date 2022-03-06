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
	"agent/pkg/timesync"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"
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
	handlerch       chan []*dto.MetricFamily
	stopCollectorCh chan bool
}

func NewCollectorWatch(conf CollectorWatchConf) *CollectorWatch {
	w := new(CollectorWatch)
	w.CollectorWatchConf = conf
	w.Watch = NewWatch()
	w.Log = w.Log.With("collector", w.Type)

	w.wg = new(sync.WaitGroup)
	w.handlerch = make(chan []*dto.MetricFamily, 1000)
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
					c.Log.Errorw("Failed to gather", zap.Error(err))

					continue
				}
				c.handlerch <- metricFamilies
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
		case metricFams := <-c.handlerch:
			for _, metricFam := range metricFams {
				out, err := proto.Marshal(metricFam)
				if err != nil {
					c.Log.Errorw("failed to marshal a metric", zap.Error(err))
					continue
				}

				// Create & emit the metric
				metricInternal := model.Message{
					Name:      string(c.Type),
					Timestamp: timesync.Default.Now().UTC().UnixMilli(),
					Type:      model.MessageType_metric,
					Body:      out,
				}

				c.Emit(metricInternal)

			}
		case <-c.StopKey:
			c.stopCollectorCh <- true
		}
	}
}
