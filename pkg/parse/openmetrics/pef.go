// Copyright 2022 Metrika Inc.
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

package openmetrics

import (
	"io"

	"agent/pkg/parse"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// OpenMetrics is a specification built upon Prometheus expositon format
// Spec: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md
// PEF spec: https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md

// ParsePEF accepts raw data in Prometheus Exposition Format.
// 'filter' is used to match only a select subset of metrics.
func ParsePEF(r io.Reader, filter parse.KeyMatcher) ([]*dto.MetricFamily, error) {
	parser := expfmt.TextParser{}
	metrics, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, err
	}
	f := []*dto.MetricFamily{}

	for name := range metrics {
		if filter == nil || filter.Match(name) {
			f = append(f, metrics[name])
		}
	}

	return f, nil
}

// PEFFilter is used as a matcher when only a subset of PEF entries are wanted.
type PEFFilter struct {
	ToMatch []string
}

// Match checks the metricName against specified set of names
func (p *PEFFilter) Match(metricName string) bool {
	for _, metric := range p.ToMatch {
		if metric == metricName {
			return true
		}
	}
	return false
}
