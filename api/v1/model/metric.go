package model

import "time"

// Metric / Metric is the base structure for all metrics sent by the agent.
type Metric struct {
	/// Timestamp represents the unix timestamp in milliseconds when the metric was created.
	Timestamp int64 `json:"timestamp"`

	/// Actionable is a hint for the consumer to use when handling this metric.
	/// `actionable=true` indicates that the metric is known to be recent.
	/// `actionable=false` indicates that the metric may be outdated, the agent does not know.
	/// When Actionable is not present, the metric is assumed to be actionable.
	Actionable bool `json:"actionable"`
}

func NewMetric(actionable bool) Metric {
	return Metric{
		Timestamp:  time.Now().UnixMilli(),
		Actionable: actionable,
	}
}
