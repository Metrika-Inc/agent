// Copyright 2016 The Prometheus Authors
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

//go:build !novmstat
// +build !novmstat

package collector

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	vmStatSubsystem = "vmstat"
)

var (
	// vmStatFields Regexp of fields to return for vmstat collector.
	vmStatFields = "^(oom_kill|pgpg|pswp|pg.*fault).*"
)

type vmStatCollector struct {
	fieldPattern *regexp.Regexp
}

// NewvmStatCollector returns a new Collector exposing vmstat stats.
func NewvmStatCollector() (prometheus.Collector, error) {
	pattern := regexp.MustCompile(vmStatFields)
	return &vmStatCollector{
		fieldPattern: pattern,
	}, nil
}

func (c *vmStatCollector) Collect(ch chan<- prometheus.Metric) {
	file, err := os.Open(procFilePath("vmstat"))
	if err != nil {
		zap.S().Error(err)

		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			zap.S().Error(err)

			return
		}
		if !c.fieldPattern.MatchString(parts[0]) {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				prometheus.BuildFQName(namespace, vmStatSubsystem, parts[0]),
				fmt.Sprintf("/proc/vmstat information field %s.", parts[0]),
				nil, nil),
			prometheus.UntypedValue,
			value,
		)
	}
	if err := scanner.Err(); err != nil {
		zap.S().Error(err)

		return
	}
}

func (c *vmStatCollector) Describe(ch chan<- *prometheus.Desc) {
	file, err := os.Open(procFilePath("vmstat"))
	if err != nil {
		zap.S().Error(err)

		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		_, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			zap.S().Error(err)

			return
		}
		if !c.fieldPattern.MatchString(parts[0]) {
			continue
		}

		ch <- prometheus.NewDesc(
			prometheus.BuildFQName(namespace, vmStatSubsystem, parts[0]),
			fmt.Sprintf("/proc/vmstat information field %s.", parts[0]),
			nil, nil)
	}
	if err := scanner.Err(); err != nil {
		zap.S().Error(err)

		return
	}
}
