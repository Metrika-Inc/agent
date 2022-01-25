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

//go:build (darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris) && !noloadavg
// +build darwin dragonfly freebsd linux netbsd openbsd solaris
// +build !noloadavg

package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type loadavgCollector struct {
	metric []typedDesc
}

// NewLoadavgCollector returns a new Collector exposing load average stats.
func NewLoadavgCollector() (prometheus.Collector, error) {
	return &loadavgCollector{
		metric: []typedDesc{
			{prometheus.NewDesc(namespace+"_load1", "1m load average.", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(namespace+"_load5", "5m load average.", nil, nil), prometheus.GaugeValue},
			{prometheus.NewDesc(namespace+"_load15", "15m load average.", nil, nil), prometheus.GaugeValue},
		},
	}, nil
}

func (c *loadavgCollector) Collect(ch chan<- prometheus.Metric) {
	loads, err := getLoad()
	if err != nil {
		err = fmt.Errorf("couldn't get load: %w", err)
		log.Error(err)

		return
	}
	for i, load := range loads {
		log.Debug("msg", "return load", "index", i, "load", load)
		ch <- c.metric[i].mustNewConstMetric(load)
	}

	if err != nil {
		log.Error(err)

		return
	}
}

func (c *loadavgCollector) Describe(ch chan<- *prometheus.Desc) {
	loads, err := getLoad()
	if err != nil {
		err = fmt.Errorf("couldn't get load: %w", err)
		log.Error(err)

		return
	}
	for i, load := range loads {
		log.Debug("msg", "return load", "index", i, "load", load)
		ch <- c.metric[i].desc
	}

	if err != nil {
		log.Error(err)

		return
	}
}
