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

//go:build (darwin || linux || openbsd) && !nomeminfo
// +build darwin linux openbsd
// +build !nomeminfo

package collector

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	memInfoSubsystem = "memory"
)

type meminfoCollector struct{}

// NewMeminfoCollector returns a new Collector exposing memory stats.
func NewMeminfoCollector() (prometheus.Collector, error) {
	return &meminfoCollector{}, nil
}

// Update calls (*meminfoCollector).getMemInfo to get the platform specific
// memory metrics.
func (c *meminfoCollector) Collect(ch chan<- prometheus.Metric) {
	var metricType prometheus.ValueType
	memInfo, err := c.getMemInfo()
	if err != nil {
		err = fmt.Errorf("couldn't get meminfo: %w", err)
		zap.S().Error(err)

		return
	}
	for k, v := range memInfo {
		if strings.HasSuffix(k, "_total") {
			metricType = prometheus.CounterValue
		} else {
			metricType = prometheus.GaugeValue
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, memInfoSubsystem, k),
				fmt.Sprintf("Memory information field %s.", k),
				nil, nil,
			),
			metricType, v,
		)
	}
}

func (c *meminfoCollector) Describe(ch chan<- *prometheus.Desc) {
	memInfo, err := c.getMemInfo()
	if err != nil {
		err = fmt.Errorf("couldn't get meminfo: %w", err)
		zap.S().Error(err)

		return
	}
	for k := range memInfo {
		ch <- prometheus.NewDesc(
			prometheus.BuildFQName(namespace, memInfoSubsystem, k),
			fmt.Sprintf("Memory information field %s.", k),
			nil, nil,
		)
	}
}
