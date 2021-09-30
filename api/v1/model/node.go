package model

type NodeState uint8

const (
	NodeStateDown NodeState = iota
	NodeStateUp
)

type NodeHealthMetric struct {
	Metric

	State NodeState `json:"state"`
}
