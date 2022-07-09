package algorand

import (
	"fmt"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

type Algorand struct{}

const (
	DefaultAlgorandPath = "./internal/pkg/global/algorand.yml"
	protocolName        = "algorand"
)

func NewAlgorand() (*Algorand, error) {
	// load a config or create a default one
	a := &Algorand{}
	return a, nil
}

func (a *Algorand) DiscoverContainer() (*types.Container, error) {
	log := zap.S().With("blockchain", "algorand")

	// check the config first
	// heavy lifting: checking the docker, extracting PID etc. and populating a.config
	// success is basically same as calling IsConfigured() again.
	log.Warn("not implemented") // TODO: Implement

	return nil, fmt.Errorf("not implemented")
}

func (a *Algorand) IsConfigured() bool {
	log := zap.S().With("blockchain", "algorand")
	// if any of a.config.X == "" return false
	// else return true
	log.Warn("not implemented") // TODO: Implement

	return false
}

func (a *Algorand) ResetConfig() error {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return fmt.Errorf("not implemented")
}

func (d *Algorand) PEFEndpoints() []global.PEFEndpoint {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return nil
}

func (a *Algorand) ContainerRegex() []string {
	return []string{}
}

func (a *Algorand) LogEventsList() map[string]model.FromContext {
	return nil
}

func (a *Algorand) NodeLogPath() string {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return ""
}

// NodeID returns the blockchain node id
func (a *Algorand) NodeID() string {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return ""
}

// NodeType returns the blockchain node type (i.e. consensus)
func (a *Algorand) NodeType() string {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return ""
}

// NodeVersion returns the blockchain node version
func (a *Algorand) NodeVersion() string {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return ""
}

func (a *Algorand) Protocol() string {
	return protocolName
}

func (a *Algorand) Network() string {
	log := zap.S().With("blockchain", "algorand")
	log.Warn("not implemented") // TODO: Implement

	return ""
}
