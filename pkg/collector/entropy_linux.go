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

//go:build !noentropy
// +build !noentropy

package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
	"go.uber.org/zap"
)

type entropyCollector struct {
	fs              procfs.FS
	entropyAvail    *prometheus.Desc
	entropyPoolSize *prometheus.Desc
}

// NewEntropyCollector returns a new Collector exposing entropy stats.
func NewEntropyCollector() (prometheus.Collector, error) {
	fs, err := procfs.NewFS(procPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open procfs: %w", err)
	}

	return &entropyCollector{
		fs: fs,
		entropyAvail: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "entropy_available_bits"),
			"Bits of available entropy.",
			nil, nil,
		),
		entropyPoolSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "entropy_pool_size_bits"),
			"Bits of entropy pool.",
			nil, nil,
		),
	}, nil
}

func (c *entropyCollector) Collect(ch chan<- prometheus.Metric) {
	stats, err := c.fs.KernelRandom()
	if err != nil {
		err = fmt.Errorf("failed to get kernel random stats: %w", err)
		zap.S().Error(err)

		return
	}

	if stats.EntropyAvaliable == nil {
		zap.S().Errorf("couldn't get entropy_avail")

		return
	}
	ch <- prometheus.MustNewConstMetric(
		c.entropyAvail, prometheus.GaugeValue, float64(*stats.EntropyAvaliable))

	if stats.PoolSize == nil {
		zap.S().Errorf("couldn't get entropy poolsize")

		return
	}
	ch <- prometheus.MustNewConstMetric(
		c.entropyPoolSize, prometheus.GaugeValue, float64(*stats.PoolSize))
}

func (c *entropyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.entropyAvail
	ch <- c.entropyPoolSize
}
