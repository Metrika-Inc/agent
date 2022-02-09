//go:build algorand

// Code generated by protobind -type algorand; DO NOT EDIT.

package discover

// Use the following code to bootstrap a new node_example.go file:
// ```go
// //go:build example
// 
// package discover
// 
// //go:generate protobind -type example ./...
// ```

//go:generate protobind -type algorand ./...

import (
	blockchain "agent/algorand"

	"go.uber.org/zap"
)

func init() {
	var err error
	log := zap.S()

	configPath = blockchain.DefaultAlgorandPath

	proto, err = blockchain.NewAlgorand()
	if err != nil {
		log.Fatalw("failed to load protocol configuration file", zap.Error(err))
	}
}
