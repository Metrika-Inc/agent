package discover

import (
	blockhain "agent/dapper"
	"errors"
	"io/fs"

	"os"

	"go.uber.org/zap"
)

var (
	proto      NodeDiscovery
	configPath string
)

type NodeDiscovery interface {
	Discover() error
	IsConfigured() bool
}

func AutoConfig(reset bool) {
	if reset {
		ResetConfig()
	}
	log := zap.S()
	var err error
	proto, err = blockhain.NewDapper()
	if err != nil {
		panic(err)
	}
	if ok := proto.IsConfigured(); ok {
		log.Info("protocol configuration OK")
		return
	}

	if err := proto.Discover(); err != nil {
		log.Fatalw("failed to automatically discover protocol configuration", zap.Error(err))
	}
}

// ResetConfig removes the protocol's configuration files.
// Allows discovery process to begin anew.
func ResetConfig() {
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			zap.S().Errorw("failed to remove a config file", zap.Error(err))
		}
	}
}
