package model

import (
	"agent/api/v1/model"
)

type NewBlockMetric struct {
	model.Metric

	Round uint64 `json:"round"`
}
