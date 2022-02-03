package discover

import (
	"agent/algorand"
	"agent/dapper"
	"agent/internal/pkg/global"
	"errors"
	"io/fs"
	"os"

	"go.uber.org/zap"
)

func AutoConfig(reset bool) {
	if reset {
		ResetConfig()
	}
	log := zap.S()
	var proto NodeDiscovery
	var err error

	switch global.Protocol {
	case "dapper":
		proto, err = dapper.NewDapper()
	case "algorand":
		proto, err = algorand.NewAlgorand()
	case "development":
		// NOTE: when working on go generation here we could have an error:
		// Unspecified protocol, please rebuild node agent with `make build-X`
		return
	default:
		log.Fatalw("unknown protocol", "protocol", global.Protocol)
	}

	if err != nil {
		log.Fatalw("failed to load protocol configuration file", zap.Error(err))
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
	var toRemove []string
	switch global.Protocol {
	case "dapper":
		toRemove = append(toRemove, dapper.DefaultDapperPath)
	case "algorand":
		toRemove = append(toRemove, global.DefaultAlgoPath)
	case "development":
		toRemove = append(toRemove, global.DefaultDapperPath, global.DefaultAlgoPath)
	}
	for _, item := range toRemove {
		if err := os.Remove(item); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				zap.S().Errorw("failed to remove a config file", zap.Error(err))
			}
		}
	}
}
