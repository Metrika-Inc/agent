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
