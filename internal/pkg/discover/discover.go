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
	proto      NodeDiscovery
	configPath string
)

type NodeDiscovery interface {
	Discover() error
	IsConfigured() bool
	ResetConfig() error
	ContainerRegex() []string
	LogEventsList() map[string]model.FromContext
	Hello() string
	NodeLogPath() string
}

func AutoConfig(reset bool) global.Chain {
	Init()

	log := zap.S()
	if reset {
		if err := proto.ResetConfig(); err != nil {
			log.Fatalw("failed to reset configuration", zap.Error(err))
		}
	}

	chain, ok := proto.(global.Chain)
	if !ok {
		log.Fatalw("protocol package does not implement chain interface", "protocol", global.Protocol)
	}

	if ok := proto.IsConfigured(); ok {
		log.Info("protocol configuration OK")
		return chain
	}

	if err := proto.Discover(); err != nil {
		log.Fatalw("failed to automatically discover protocol configuration", zap.Error(err))
	}

	return chain
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

func Hello() string {
	return proto.Hello()
}

func LogEventsList() map[string]model.FromContext {
	return proto.LogEventsList()
}

func ContainerRegex() []string {
	return proto.ContainerRegex()
}

func NodeLogPath() string {
	return proto.NodeLogPath()
}
