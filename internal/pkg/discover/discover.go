package discover

import (
	"agent/internal/pkg/global"
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
	ResetConfig() error
}

func AutoConfig(reset bool) global.Chain {
	log := zap.S()
	if reset {
		if err := proto.ResetConfig(); err != nil {
			log.Fatalw("failed to reset configuration", zap.Error(err))
		}
	}

	// start()

	if ok := proto.IsConfigured(); ok {
		log.Info("protocol configuration OK")
		return proto.(global.Chain)
	}

	if err := proto.Discover(); err != nil {
		log.Fatalw("failed to automatically discover protocol configuration", zap.Error(err))
	}

	return proto.(global.Chain)
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
