package watch

import (
	"agent/pkg/parse/openmetrics"
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	dto "github.com/prometheus/client_model/go"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestDtoToOpenMetrics(t *testing.T) {
	res, err := ioutil.ReadFile("../../internal/pkg/testutils/testdata/metricfamily/collector_data.jsonl")
	require.NoError(t, err, "failed to read file")
	res = bytes.Trim(res, "\n")
	metricFams := []*dto.MetricFamily{}
	for _, item := range bytes.Split(res, []byte{'\n'}) {
		buf := bytes.NewBuffer(item)
		m := &dto.MetricFamily{}
		err := jsonpb.Unmarshal(buf, m)
		require.NoError(t, err)
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
				openExemplar := openGauge.GetExemplar()
				if openExemplar != nil {
					exemplar := gauge.GetExemplar()
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
				continue
			case io_prometheus_client.MetricType_SUMMARY:
				continue
			}
		}
	}
}

func BenchmarkDtoToOpenMetrics(b *testing.B) {
	res, err := ioutil.ReadFile("../parse/openmetrics/testdata/happy/pef_algorand")
	require.NoError(b, err, "failed to read file")
	buf := bytes.NewBuffer(res)
	metricFams, err := openmetrics.ParsePEF(buf, nil)
	require.NoError(b, err)
	for j := range metricFams {
		f := func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dtoToOpenMetrics(metricFams[j])
			}
		}
		b.Run(fmt.Sprintf("metric %d", j), f)

	}
}

// TODO: get some metrics from collectors, or find histograms/summaries somewhere
