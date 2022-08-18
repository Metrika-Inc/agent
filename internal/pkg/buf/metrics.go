package buf

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

	// BufferInsertDuration ...
	BufferInsertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "agent_buffer_insert_duration_seconds",
		Help:    "Histogram of buffer insert() duration in seconds",
		Buckets: buckets,
	})

	// BufferGetDuration ...
	BufferGetDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "agent_buffer_get_duration_seconds",
		Help:    "Histogram of buffer get() duration in seconds",
		Buckets: buckets,
	})

	// BufferDrainDuration ...
	BufferDrainDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "agent_buffer_drain_duration_seconds",
		Help:    "Histogram of buffer drain() duration in seconds",
		Buckets: buckets,
	})

	// BufferSize ...
	BufferSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "agent_buffer_total_size", Help: "The total size of bufferred data",
	})

	MetricsDropCnt = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_metrics_drop_total_count", Help: "The total number of metrics dropped",
	})
)
