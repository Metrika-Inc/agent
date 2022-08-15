package transport

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PlatformPublishErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_platform_publish_errors_total", Help: "The total number of errors while sending data to the platform.",
	})

	MetricsPublishedCnt = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_metrics_published_total_count", Help: "The total number of metrics successfully published",
	})
)
