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
package collector

import (
	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"encoding/json"
	"errors"
	"sync"
	"time"

	. "agent/pkg/watch"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
)

// Namespace defines the common namespace to be used by all metrics.
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

// const (
// 	defaultEnabled  = true
// 	defaultDisabled = false
// )

// var (
// 	// factories              = make(map[string]func(logger log.Logger) (Collector, error))
// 	initiatedCollectorsMtx = sync.Mutex{}
// 	initiatedCollectors    = make(map[string]Collector)
// 	collectorState         = make(map[string]*bool)
// 	forcedCollectors       = map[string]bool{} // collectors which have been explicitly enabled or disabled
// )

// func registerCollector(collector string, isDefaultEnabled bool, factory func(logger log.Logger) (Collector, error)) {
// 	var helpDefaultState string
// 	if isDefaultEnabled {
// 		helpDefaultState = "enabled"
// 	} else {
// 		helpDefaultState = "disabled"
// 	}

// 	flagName := fmt.Sprintf("collector.%s", collector)
// 	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
// 	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)

// 	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Action(collectorFlagAction(collector)).Bool()
// 	collectorState[collector] = flag

// 	factories[collector] = factory
// }

// // NodeCollector implements the prometheus.Collector interface.
// type NodeCollector struct {
// 	Collectors map[string]Collector
// 	logger     log.Logger
// }

// DisableDefaultCollectors sets the collector state to false for all collectors which
// have not been explicitly enabled on the command line.
// func DisableDefaultCollectors() {
// 	for c := range collectorState {
// 		if _, ok := forcedCollectors[c]; !ok {
// 			*collectorState[c] = false
// 		}
// 	}
// }

// collectorFlagAction generates a new action function for the given collector
// to track whether it has been explicitly enabled or disabled from the command line.
// A new action function is needed for each collector flag because the ParseContext
// does not contain information about which flag called the action.
// See: https://github.com/alecthomas/kingpin/issues/294
// func collectorFlagAction(collector string) func(ctx *kingpin.ParseContext) error {
// 	return func(ctx *kingpin.ParseContext) error {
// 		forcedCollectors[collector] = true
// 		return nil
// 	}
// }

// NewNodeCollector creates a new NodeCollector.
// func NewNodeCollector(logger log.Logger, filters ...string) (*NodeCollector, error) {
// 	f := make(map[string]bool)
// 	for _, filter := range filters {
// 		enabled, exist := collectorState[filter]
// 		if !exist {
// 			return nil, fmt.Errorf("missing collector: %s", filter)
// 		}
// 		if !*enabled {
// 			return nil, fmt.Errorf("disabled collector: %s", filter)
// 		}
// 		f[filter] = true
// 	}
// 	collectors := make(map[string]Collector)
// 	initiatedCollectorsMtx.Lock()
// 	defer initiatedCollectorsMtx.Unlock()
// 	for key, enabled := range collectorState {
// 		if !*enabled || (len(f) > 0 && !f[key]) {
// 			continue
// 		}
// 		if collector, ok := initiatedCollectors[key]; ok {
// 			collectors[key] = collector
// 		} else {
// 			collector, err := factories[key](log.With(logger, "collector", key))
// 			if err != nil {
// 				return nil, err
// 			}
// 			collectors[key] = collector
// 			initiatedCollectors[key] = collector
// 		}
// 	}
// 	return &NodeCollector{Collectors: collectors, logger: logger}, nil
// }

// Describe implements the prometheus.Collector interface.
// func (n NodeCollector) Describe(ch chan<- *prometheus.Desc) {
// 	ch <- scrapeDurationDesc
// 	ch <- scrapeSuccessDesc
// }

// // Collect implements the prometheus.Collector interface.
// func (n NodeCollector) Collect(ch chan<- prometheus.Metric) {
// 	wg := sync.WaitGroup{}
// 	wg.Add(len(n.Collectors))
// 	for name, c := range n.Collectors {
// 		go func(name string, c Collector) {
// 			execute(name, c, ch, n.logger)
// 			wg.Done()
// 		}(name, c)
// 	}
// 	wg.Wait()
// }

func execute(name string, c Collector, ch chan<- prometheus.Metric, logger log.Logger) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if IsNoDataError(err) {
			logger.Debug("msg", "collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			logger.Error("msg", "collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
		success = 0
	} else {
		logger.Debug("msg", "collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

type typedDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}

// ErrNoData indicates the collector found no data to collect, but had no other error.
var ErrNoData = errors.New("collector returned no data")

func IsNoDataError(err error) bool {
	return err == ErrNoData
}

type CollectorWatchConf struct {
	Type      global.CollectorType
	Collector Collector
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
