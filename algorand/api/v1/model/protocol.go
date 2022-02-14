package model

import (
	"agent/api/v1/model"
)

type NewBlockMetric struct {
	model.Message

	Round uint64 `json:"round"`
}
