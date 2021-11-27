package publisher

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PlatformHTTPRequestErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_platform_http_errors_total", Help: "The total number of HTTP errors received from the platform.",
	})

	MetricsPublishedCnt = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_metrics_published_total_count", Help: "The total number of metrics successfully published",
	})

	MetricsDropCnt = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_metrics_drop_total_count", Help: "The total number of metrics dropped",
	})
)
