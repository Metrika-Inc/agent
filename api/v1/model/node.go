package model

// type NodeState uint8

type NodeHealthMetric struct {
	Metric

	State NodeState `json:"state"`
}
