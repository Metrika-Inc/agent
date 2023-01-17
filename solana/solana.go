package solana

import (
	"io"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types"
)

const (
	// solanaValidator value to use as node role
	solanaValidator = "solana-validator"

	// protocolName blockchain protocol name
	protocolName = "solana"

	// DefaultSolanaPath default Solana configuration path
	DefaultSolanaPath = "/etc/metrikad/configs/solana.yml"

	// defaultInfluxListenAddr default listen address for Influx reverse proxy
	defaultInfluxListenAddr = "127.0.0.1:8086"

	// defaultInfluxUpstreamURL default upstream URL to proxy InfluxDB write requests
	defaultInfluxUpstreamURL = "https://metrics.solana.com:8086"
)

var defaultRuntimeWatcherInfluxConf = &global.WatchConfig{
	Type:        "influx",
	ListenAddr:  defaultInfluxListenAddr,
	UpstreamURL: defaultInfluxUpstreamURL,
}

// Solana object to hold node state.
type Solana struct {
	*solanaConfig
}

// IsConfigured noop
func (s *Solana) IsConfigured() bool {
	return true
}

// ResetConfig noop
func (s *Solana) ResetConfig() error {
	return nil
}

// PEFEndpoints noop
func (s *Solana) PEFEndpoints() []global.PEFEndpoint {
	return []global.PEFEndpoint{}
}

// ContainerRegex noop
func (s *Solana) ContainerRegex() []string {
	return []string{}
}

// LogEventsList noop
func (s *Solana) LogEventsList() map[string]model.FromContext {
	return map[string]model.FromContext{}
}

// LogWatchEnabled noop
func (s *Solana) LogWatchEnabled() bool {
	return false
}

// NodeLogPath noop
func (s *Solana) NodeLogPath() string {
	return ""
}

// NodeID noop
func (s *Solana) NodeID() string {
	return ""
}

// NodeRole returns solana-validator
func (s *Solana) NodeRole() string {
	return solanaValidator
}

// NodeVersion noop
func (s *Solana) NodeVersion() string {
	return ""
}

// DiscoverContainer noop
func (s *Solana) DiscoverContainer() (*types.Container, error) {
	return nil, nil
}

// Protocol protocol name to use
func (s *Solana) Protocol() string {
	return protocolName
}

// Network nonet
func (s *Solana) Network() string {
	return ""
}

// Reconfigure noop
func (s *Solana) Reconfigure() error {
	return nil
}

// ReconfigureByDockerContainer nooop
func (s *Solana) ReconfigureByDockerContainer(container *types.Container, reader io.ReadCloser) error {
	return nil
}

// ReconfigureBySystemdUnit noop
func (s *Solana) ReconfigureBySystemdUnit(unit *dbus.UnitStatus, reader io.ReadCloser) error {
	return nil
}

// SetRunScheme noop
func (s *Solana) SetRunScheme(global.NodeRunScheme) {
	return
}

// SetDockerContainer noop
func (s *Solana) SetDockerContainer(*types.Container) {
	return
}

// SetSystemdService noop
func (s *Solana) SetSystemdService(*dbus.UnitStatus) {
	return
}

// DiscoveryDeactivated enabled by default for Solana
func (s *Solana) DiscoveryDeactivated() bool {
	return true
}

// RuntimeDisableFingerprintValidation disabled by default for Solana
func (s *Solana) RuntimeDisableFingerprintValidation() bool {
	return true
}

// RuntimeWatchersInflux default influx watcher configuration
func (s *Solana) RuntimeWatchersInflux() *global.WatchConfig {
	return defaultRuntimeWatcherInfluxConf
}

// PlatformEnabled enabled by default for Solana
func (s *Solana) PlatformEnabled() bool {
	return false
}

// ConfigUpdateCh returns the channel used to emit configuration updates
func (s *Solana) ConfigUpdateCh() chan global.ConfigUpdate {
	return nil
}

// NewSolana returns an implementation of the global.Chain interface for Solana.
func NewSolana() (*Solana, error) {
	return &Solana{
		solanaConfig: newSolanaConfig(protocolName),
	}, nil
}
