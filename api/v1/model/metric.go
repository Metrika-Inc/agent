package model

import (
	"time"
	"unsafe"
)

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

// MetricBatch slice of metrics to be published at once
type MetricBatch []MetricPlatform

// Bytes returns the total size of the batch in bytes
func (a MetricBatch) Bytes() uint {
	bytesDec := uint(0)
	for _, m := range a {
		bytesDec += m.Bytes()
	}

	return bytesDec
}

func (a *MetricBatch) Add(msg MetricPlatform) {
	*a = append(*a, msg)
}

func (a *MetricBatch) Clear() {
	*a = (*a)[:0]
}

// MetricPlatform wraps a serialized Metric with context.
type MetricPlatform struct {
	// Timestamp UNIX timestamp in milliseconds at metric creation time
	Timestamp int64 `json:"timestamp"`

	// NodeState node state at metric creation time
	NodeState NodeState `json:"node_state"`

	// Type metric type name
	Type string `json:"type"`

	// Body serialized Metric
	Body []byte `json:"body"`
}

// Bytes estimates the total size of the metric in bytes.
func (m MetricPlatform) Bytes() uint {
	return uint(unsafe.Sizeof(m)) + uint(len(m.Type)+len(m.Body)) + uint(8-len(m.Type)%8)%8 + uint(8-len(m.Body)%8)%8
}

func (m Message) Bytes() uint {
	return uint(unsafe.Sizeof(m)) + 20 + uint(len(m.Name)+len(m.Data)) + uint(8-len(m.Name)%8)%8 + uint(8-len(m.Data)%8)%8
}

// MetrikaMessage wraps a batch of metrics with agent context.
type MetrikaMessage struct {
	// UUID agent instance UUID
	UUID string `json:"uid" binding:"required,uuid"`

	// Data batch of metrics
	Data []MetricPlatform `json:"data"`
}
