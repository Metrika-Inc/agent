package global

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsDropCnt tracked metrics dropped by agent
var MetricsDropCnt = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agent_metrics_drop_total_count", Help: "The total number of metrics dropped",
}, []string{"reason"})
