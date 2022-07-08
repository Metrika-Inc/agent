package watch

import (
	"errors"
	"time"

	"agent/api/v1/model"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func dtoToOpenMetrics(in *dto.MetricFamily) (*model.MetricFamily, error) {
	out := &model.MetricFamily{
		Name:    in.GetName(),
		Help:    in.GetHelp(),
		Metrics: make([]*model.Metric, len(in.Metric)),
	}
	for i, metric := range in.Metric {
		labels := metric.GetLabel()
		out.Metrics[i] = &model.Metric{
			Labels: make([]*model.Label, len(labels)),
		}
		for j, metric := range labels {
			out.Metrics[i].Labels[j] = &model.Label{
				Name:  metric.GetName(),
				Value: metric.GetValue(),
			}
		}

		if out.Metrics[i].GetMetricPoints() == nil {
			out.Metrics[i].MetricPoints = []*model.MetricPoint{}
		}

		out.Metrics[i].MetricPoints = append(out.Metrics[i].MetricPoints, &model.MetricPoint{
			Timestamp: timestamppb.New(time.UnixMilli(metric.GetTimestampMs()).UTC()),
		})

		switch in.GetType() {
		case dto.MetricType_GAUGE:
			out.Type = model.MetricType_GAUGE
			gauge := metric.GetGauge()
			if gauge == nil {
				return nil, errors.New("expected gauge to not be nil")
			}
			value := &model.MetricPoint_GaugeValue{
				GaugeValue: &model.GaugeValue{
					Value: &model.GaugeValue_DoubleValue{
						DoubleValue: gauge.GetValue(),
					},
				},
			}
			out.Metrics[i].MetricPoints[0].Value = value
		case dto.MetricType_COUNTER:
			out.Type = model.MetricType_COUNTER
			counter := metric.GetCounter()
			if counter == nil {
				return nil, errors.New("expected counter to not be nil")
			}
			value := &model.MetricPoint_CounterValue{
				CounterValue: &model.CounterValue{
					Total: &model.CounterValue_DoubleValue{
						DoubleValue: counter.GetValue(),
					},
				},
			}
			if exemplar := counter.GetExemplar(); exemplar != nil {
				labels := exemplar.GetLabel()
				value.CounterValue.Exemplar = &model.Exemplar{
					Value:     exemplar.GetValue(),
					Timestamp: exemplar.GetTimestamp(),
					Label:     make([]*model.Label, len(labels)),
				}
				for j, label := range labels {
					value.CounterValue.Exemplar.Label[j].Name = label.GetName()
					value.CounterValue.Exemplar.Label[j].Value = label.GetValue()
				}
			}
			out.Metrics[i].MetricPoints[0].Value = value
		case dto.MetricType_HISTOGRAM:
			out.Type = model.MetricType_HISTOGRAM
			histogram := metric.GetHistogram()
			if histogram == nil {
				return nil, errors.New("expected histogram to not be nil")
			}

			buckets := histogram.GetBucket()
			value := &model.MetricPoint_HistogramValue{
				HistogramValue: &model.HistogramValue{
					Sum: &model.HistogramValue_DoubleValue{
						DoubleValue: histogram.GetSampleSum(),
					},
					Count:   histogram.GetSampleCount(),
					Buckets: make([]*model.HistogramValue_Bucket, len(buckets)),
				},
			}
			for i, bucket := range buckets {
				value.HistogramValue.Buckets[i] = &model.HistogramValue_Bucket{
					Count:      bucket.GetCumulativeCount(),
					UpperBound: bucket.GetUpperBound(),
				}
				if exemplar := bucket.GetExemplar(); exemplar != nil {
					labels := exemplar.GetLabel()
					value.HistogramValue.Buckets[i].Exemplar = &model.Exemplar{
						Value:     exemplar.GetValue(),
						Timestamp: exemplar.GetTimestamp(),
						Label:     make([]*model.Label, len(labels)),
					}
					for j, label := range labels {
						value.HistogramValue.Buckets[i].Exemplar.Label[j].Name = label.GetName()
						value.HistogramValue.Buckets[i].Exemplar.Label[j].Value = label.GetValue()
					}
				}
			}

			out.Metrics[i].MetricPoints[0].Value = value
		case dto.MetricType_SUMMARY:
			out.Type = model.MetricType_SUMMARY
			summary := metric.GetSummary()
			if summary == nil {
				return nil, errors.New("expected summary to not be nil")
			}
			quantiles := summary.GetQuantile()
			value := &model.MetricPoint_SummaryValue{
				SummaryValue: &model.SummaryValue{
					Sum: &model.SummaryValue_DoubleValue{
						DoubleValue: summary.GetSampleSum(),
					},
					Count:    summary.GetSampleCount(),
					Quantile: make([]*model.SummaryValue_Quantile, len(quantiles)),
				},
			}

			for j, quantile := range quantiles {
				value.SummaryValue.Quantile[j] = &model.SummaryValue_Quantile{
					Quantile: quantile.GetQuantile(),
					Value:    quantile.GetValue(),
				}
			}
			out.Metrics[i].MetricPoints[0].Value = value
		}
	}

	return out, nil
}

func setDTOMetriFamilyTimestamp(t time.Time, metricFamilies ...*dto.MetricFamily) {
	ts := t.UnixMilli()
	for _, mf := range metricFamilies {
		for _, dtometric := range mf.GetMetric() {
			dtometric.TimestampMs = &ts
		}
	}
}
