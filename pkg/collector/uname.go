// Copyright 2019 The Prometheus Authors
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

//go:build (darwin || freebsd || openbsd || linux) && !nouname
// +build darwin freebsd openbsd linux
// +build !nouname

package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

var unameDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "uname", "info"),
	"Labeled system information as provided by the uname system call.",
	[]string{
		"sysname",
		"release",
		"version",
		"machine",
		"nodename",
		"domainname",
	},
	nil,
)

type unameCollector struct{}
type uname struct {
	SysName    string
	Release    string
	Version    string
	Machine    string
	NodeName   string
	DomainName string
}

// NewUnameCollector returns new unameCollector.
func NewUnameCollector() (prometheus.Collector, error) {
	return &unameCollector{}, nil
}

func (c *unameCollector) Collect(ch chan<- prometheus.Metric) {
	uname, err := getUname()
	if err != nil {
		log.Error(err)

		return
	}

	ch <- prometheus.MustNewConstMetric(unameDesc, prometheus.GaugeValue, 1,
		uname.SysName,
		uname.Release,
		uname.Version,
		uname.Machine,
		uname.NodeName,
		uname.DomainName,
	)
}

func (c *unameCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- unameDesc
}
