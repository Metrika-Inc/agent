package discover

import (
	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"errors"
	"io/fs"

	"os"

	"go.uber.org/zap"
)

var (
	chain      global.Chain
	configPath string
)

func AutoConfig(reset bool) global.Chain {
	Init()

	log := zap.S()
	if reset {
		if err := chain.ResetConfig(); err != nil {
			log.Fatalw("failed to reset configuration", zap.Error(err))
		}
	}

	chn, ok := chain.(global.Chain)
	if !ok {
		log.Fatalw("protocol package does not implement chain interface", "protocol", global.Blockchain)
	}

	if ok := chain.IsConfigured(); ok {
		log.Info("protocol configuration OK")
		return chn
	}

	if err := chain.Discover(); err != nil {
		log.Fatalw("failed to automatically discover protocol configuration", zap.Error(err))
	}

	return chn
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

func LogEventsList() map[string]model.FromContext {
	return chain.LogEventsList()
}

func ContainerRegex() []string {
	return chain.ContainerRegex()
}

func NodeLogPath() string {
	return chain.NodeLogPath()
}
