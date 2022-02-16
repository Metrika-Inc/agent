package model

import (
	"unsafe"

	dto "github.com/prometheus/client_model/go"
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
	dataSize := uint(unsafe.Sizeof(m))
	// Message.Name
	dataSize += stringBytes(m.Name)

	if mf := m.GetMetric(); mf != nil {
		// Struct size
		dataSize += uint(unsafe.Sizeof(dto.MetricFamily{}))
		// MetricFamily.Name, MetricFamily.Help
		dataSize += stringBytes(mf.GetName()) + stringBytes(mf.GetHelp())
		// MetricFamily.Type is int32, pad to 8 bytes
		dataSize += 8

		// MetricFamily.[]Metric
		metricStructSize := uint(unsafe.Sizeof(dto.Metric{}))
		for _, metric := range mf.Metric {
			dataSize += metricStructSize
			// Metric.[]LabelPair
			for _, label := range metric.Label {
				dataSize += labelPairBytes(label)
			}
			// Metric.Gauge
			if g := metric.GetGauge(); g != nil {
				dataSize += uint(unsafe.Sizeof(g))
				// Gauge.Value + footer
				dataSize += 8 + protoBytes(len(g.XXX_unrecognized))
			}
			// Metric.Counter
			if c := metric.GetCounter(); c != nil {
				dataSize += uint(unsafe.Sizeof(c))
				// Coutner.Value
				dataSize += 8
				if e := c.GetExemplar(); e != nil {
					dataSize += exemplarBytes(e)
				}
			}
			// Metric.Summary
			if s := metric.GetSummary(); s != nil {
				dataSize += uint(unsafe.Sizeof(s))
				// Summary.SampleCount + Summary.SampleSum
				dataSize += 16
				// Summary.Quantile
				for _, q := range s.GetQuantile() {
					dataSize += uint(unsafe.Sizeof(q))
					// Quantile.Quantile + Quantile.Value + footer
					dataSize += 16 + protoBytes(len(q.XXX_unrecognized))
				}
				// footer
				dataSize += protoBytes(len(s.XXX_unrecognized))
			}
			// Metric.Untyped
			if u := metric.GetUntyped(); u != nil {
				dataSize += uint(unsafe.Sizeof(u))
				// Untyped.Value + footer
				dataSize += 8 + protoBytes(len(u.XXX_unrecognized))
			}
			// Metric.Histogram
			if h := metric.GetHistogram(); h != nil {
				dataSize += uint(unsafe.Sizeof(h))
				// Histogram.SampleCount + Histogram.SampleSum
				dataSize += 16
				// Histogram.[]Bucket
				for _, b := range h.GetBucket() {
					dataSize += uint(unsafe.Sizeof(b))
					// Bucket.CumulativeCount + Bucket.UpperBound
					dataSize += 16
					// Bucket.Exemplar + footer
					dataSize += exemplarBytes(b.Exemplar) + protoBytes(len(b.XXX_unrecognized))
				}
				// footer
				dataSize += protoBytes(len(h.XXX_unrecognized))
			}
			// Metric.TimestampMs + footer
			dataSize += 8 + protoBytes(len(metric.XXX_unrecognized))
		}
		// footer
		dataSize += protoBytes(len(mf.XXX_unrecognized))
	}
	if e := m.GetEvent(); e != nil {
		dataSize += uint(unsafe.Sizeof(e))
	}
	dataSize += protoBytes(len(m.XXX_unrecognized))
	return dataSize
}

// multipleOf8 is a helper function for padding.
// It rounds up to the next multiple of 8.
// If length is zero, it returns zero.
func multipleOf8(length int) uint {
	if length == 0 {
		return 0
	}
	return uint((length + 7) & -8)
}

// protoBytes calculates the value of XXX_unrecognized that is
// generated in every protobuf model.
// It takes the argument of len(XXX_unrecognized).
// Note: other values are automatically calculated in unsafe.Sizeof operation.
func protoBytes(unrecognizedLen int) uint {
	return uint(unrecognizedLen) + multipleOf8(unrecognizedLen)
}

func exemplarBytes(e *dto.Exemplar) uint {
	if e == nil {
		return 0
	}
	ret := uint(unsafe.Sizeof(e))
	// Exemplar.LabelPair
	for _, label := range e.Label {
		ret += labelPairBytes(label)
	}
	// Exemplar.Value
	ret += 8
	// Exemplar.Timestamp
	if t := e.GetTimestamp(); t != nil {
		ret += uint(unsafe.Sizeof(e))
	}

	return ret + protoBytes(len(e.XXX_unrecognized))
}

func stringBytes(s string) uint {
	return uint(len(s)) + multipleOf8(len(s))
}

func labelPairBytes(label *dto.LabelPair) uint {
	ret := uint(unsafe.Sizeof(label))
	// LabelPair.Name and LabelPair.Value
	ret += stringBytes(label.GetName()) + stringBytes(label.GetValue())

	return ret + protoBytes(len(label.XXX_unrecognized))
}

// MetrikaMessage wraps a batch of metrics with agent context.
type MetrikaMessage struct {
	// UUID agent instance UUID
	UUID string `json:"uid" binding:"required,uuid"`

	// Data batch of metrics
	Data []MetricPlatform `json:"data"`
}
