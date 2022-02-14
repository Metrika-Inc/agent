package model

import (
	"unsafe"
)

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
	// var dataSize uint
	// if mf := m.GetMetric(); mf != nil {
	// 	dataSize = uint(len(*mf.Name)) + uint(8-len(*mf.Name)%8)%8 +
	// 		uint(len(*mf.Help)) + uint(8-len(*mf.Help)%8)%8 +
	// 		4
	// 	metricStructSize := uint(unsafe.Sizeof(Metric{}))
	// 	// metricGaugeSize := uint(unsafe.Sizeof(Gauge{}))
	// 	for _, metric := range mf.Metric {
	// 		dataSize += metricStructSize

	// 	}
	// }
	// return uint(unsafe.Sizeof(m)) + 20 + uint(len(m.Name)+len(m.Data)) + uint(8-len(m.Name)%8)%8 + uint(8-len(m.Data)%8)%8
	return 0
}

// MetrikaMessage wraps a batch of metrics with agent context.
type MetrikaMessage struct {
	// UUID agent instance UUID
	UUID string `json:"uid" binding:"required,uuid"`

	// Data batch of metrics
	Data []MetricPlatform `json:"data"`
}
