package model

import (
	. "agent/api/v1/model"
)

type NewBlockMetric struct {
	Metric

	Round uint64 `json:"round"`
}
