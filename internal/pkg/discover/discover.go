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
}

func AutoConfig(reset bool) global.Chain {
	if reset {
		ResetConfig()
	}

	start()

	log := zap.S()

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
	if err := os.Remove(global.DefaultChainPath[global.Protocol]); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			zap.S().Errorw("failed to remove a config file", zap.Error(err))
		}
	}
}
