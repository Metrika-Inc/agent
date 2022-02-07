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
