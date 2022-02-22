//go:build dapper

// Code generated by protobind -type dapper; DO NOT EDIT.

package discover

// Use the following code to bootstrap a new node_example.go file:
// ```go
// //go:build example
// 
// package discover
// 
// //go:generate protobind -type example ./...
// ```

//go:generate protobind -type dapper ./...

import (
	blockhain "agent/dapper"
	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

func init() {
	var err error
	log := zap.S()

	configPath = global.DefaultDapperPath

	proto, err = blockhain.NewDapper()
	if err != nil {
		log.Fatalw("failed to load protocol configuration file", zap.Error(err))
	}
}

