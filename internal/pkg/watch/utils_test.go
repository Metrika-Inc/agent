package watch

import (
	"bytes"
	"io/ioutil"
	"math"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	dto "github.com/prometheus/client_model/go"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestDtoToOpenMetrics(t *testing.T) {
	res, err := ioutil.ReadFile("../testutils/testdata/metricfamily/collector_data.jsonl")
	require.NoError(t, err, "failed to read file")
	res = bytes.Trim(res, "\n")
	metricFams := []*dto.MetricFamily{}
	for i, item := range bytes.Split(res, []byte{'\n'}) {
		buf := bytes.NewBuffer(item)
		m := &dto.MetricFamily{}
		err := jsonpb.Unmarshal(buf, m)
		require.NoError(t, err, "error on line %d", i+1)
		metricFams = append(metricFams, m)
	}
	require.NoError(t, err)

	for _, metricFam := range metricFams {
		openMetricFam, err := dtoToOpenMetrics(metricFam)
		require.NoError(t, err)
		require.Equal(t, metricFam.GetName(), openMetricFam.GetName())
		require.Equal(t, metricFam.GetHelp(), openMetricFam.GetHelp())
		openMetrics := openMetricFam.GetMetrics()
		metrics := metricFam.GetMetric()
		require.Len(t, openMetrics, len(metrics))
		for i, metric := range metrics {
			openMetricPoints := openMetrics[i].GetMetricPoints()
			require.Len(t, openMetricPoints, 1)
			openMetricPoint := openMetricPoints[0]
			expectedTs := time.UnixMilli(metric.GetTimestampMs()).UTC()
			openTimestamp := openMetricPoint.GetTimestamp().AsTime()
			require.Equal(t, expectedTs, openTimestamp,
				"expected %+v\ngot %+v, [%d]", expectedTs, openTimestamp, i)
			switch metricFam.GetType() {
			case io_prometheus_client.MetricType_GAUGE:
				openGauge := openMetricPoints[0].GetGaugeValue()
				require.NotNil(t, openGauge)
				gauge := metric.GetGauge()
				require.NotNil(t, openGauge)
				require.Equal(t, gauge.GetValue(), openGauge.GetDoubleValue())
			case io_prometheus_client.MetricType_COUNTER:
				openGauge := openMetricPoints[0].GetCounterValue()
				require.NotNil(t, openGauge)
				gauge := metric.GetCounter()
				require.NotNil(t, openGauge)
				require.Equal(t, gauge.GetValue(), openGauge.GetDoubleValue())
				exemplar := gauge.GetExemplar()
				openExemplar := openGauge.GetExemplar()
				if exemplar != nil {
					require.Equal(t, exemplar.GetValue(), openExemplar.GetValue())
					require.True(t, proto.Equal(exemplar.GetTimestamp(), openExemplar.GetTimestamp()))
					openExemplarLabels := openExemplar.GetLabel()
					exemplarLabels := exemplar.GetLabel()
					require.Len(t, openExemplarLabels, len(exemplarLabels))
					for j := range exemplarLabels {
						require.Equal(t, exemplarLabels[i].GetName(), openExemplarLabels[j].GetName())
						require.Equal(t, exemplarLabels[i].GetName(), openExemplarLabels[i].GetName())
					}
				} else {
					require.Nil(t, openExemplar)
				}
			case io_prometheus_client.MetricType_HISTOGRAM:
				openHistogram := openMetricPoints[0].GetHistogramValue()
				require.NotNil(t, openHistogram)
				histogram := metric.GetHistogram()
				require.NotNil(t, histogram)
				require.Equal(t, histogram.GetSampleCount(), openHistogram.Count)
				if math.IsNaN(histogram.GetSampleSum()) {
					require.True(t, math.IsNaN(openHistogram.GetDoubleValue()))
				} else {
					require.Equal(t, histogram.GetSampleSum(), openHistogram.GetDoubleValue())
				}
				buckets := histogram.GetBucket()
				openBuckets := openHistogram.GetBuckets()

				require.Len(t, openBuckets, len(buckets))
				for i, bucket := range buckets {
					require.Equal(t, bucket.GetCumulativeCount(), openBuckets[i].GetCount())
					require.Equal(t, bucket.GetUpperBound(), openBuckets[i].GetUpperBound())
					exemplar := bucket.GetExemplar()
					openExemplar := openBuckets[i].GetExemplar()
					if exemplar != nil {
						require.Equal(t, exemplar.GetValue(), openExemplar.GetValue())
						require.True(t, proto.Equal(openExemplar.GetTimestamp(), exemplar.GetTimestamp()))
						exemplarLabels := exemplar.GetLabel()
						openExemplarLabels := openExemplar.GetLabel()

						require.Len(t, openExemplarLabels, len(exemplarLabels))
						for j := range exemplarLabels {
							require.Equal(t, exemplarLabels[i].GetName(), openExemplarLabels[j].GetName())
							require.Equal(t, exemplarLabels[i].GetName(), openExemplarLabels[i].GetName())
						}
					} else {
						require.Nil(t, openExemplar)
					}
				}
			case io_prometheus_client.MetricType_SUMMARY:
				openSummary := openMetricPoints[0].GetSummaryValue()
				require.NotNil(t, openSummary)
				summary := metric.GetSummary()
				require.NotNil(t, summary)
				require.Equal(t, summary.GetSampleCount(), openSummary.GetCount())
				require.Equal(t, summary.GetSampleSum(), openSummary.GetDoubleValue())
				quantiles := summary.GetQuantile()
				openQuantiles := openSummary.GetQuantile()
				require.Len(t, openQuantiles, len(quantiles))
				for i, quantile := range quantiles {
					require.Equal(t, quantile.GetQuantile(), openQuantiles[i].GetQuantile())
					require.Equal(t, quantile.GetValue(), openQuantiles[i].GetValue())
				}
			}
		}
	}
}

// TODO: get some metrics from collectors, or find histograms/summaries somewhere
