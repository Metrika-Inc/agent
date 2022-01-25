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

//go:build !notime
// +build !notime

package collector

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type timeCollector struct {
	now                   typedDesc
	zone                  typedDesc
	clocksourcesAvailable typedDesc
	clocksourceCurrent    typedDesc
}

// NewTimeCollector returns a new Collector exposing the current system time in
// seconds since epoch.
func NewTimeCollector() (prometheus.Collector, error) {
	const subsystem = "time"
	return &timeCollector{
		now: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "seconds"),
			"System time in seconds since epoch (1970).",
			nil, nil,
		), prometheus.GaugeValue},
		zone: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "zone_offset_seconds"),
			"System time zone offset in seconds.",
			[]string{"time_zone"}, nil,
		), prometheus.GaugeValue},
		clocksourcesAvailable: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "clocksource_available_info"),
			"Available clocksources read from '/sys/devices/system/clocksource'.",
			[]string{"device", "clocksource"}, nil,
		), prometheus.GaugeValue},
		clocksourceCurrent: typedDesc{prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "clocksource_current_info"),
			"Current clocksource read from '/sys/devices/system/clocksource'.",
			[]string{"device", "clocksource"}, nil,
		), prometheus.GaugeValue},
	}, nil
}

func (c *timeCollector) Collect(ch chan<- prometheus.Metric) {
	now := time.Now()
	nowSec := float64(now.UnixNano()) / 1e9
	zone, zoneOffset := now.Zone()

	ch <- c.now.mustNewConstMetric(nowSec)
	ch <- c.zone.mustNewConstMetric(float64(zoneOffset), zone)
	c.update(ch)

	return
}

func (c *timeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.now.desc
	ch <- c.zone.desc
	ch <- c.clocksourcesAvailable.desc
	ch <- c.clocksourceCurrent.desc
}
