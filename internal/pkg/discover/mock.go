// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discover

import (
	"io"
	"sync"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types"
)

// MockBlockchain ...
type MockBlockchain struct {
	logWatchEnabled bool
	sync.RWMutex
}

// NewMockBlockchain creates a new instance of Mock Blockchain. Used for tests.
func NewMockBlockchain() *MockBlockchain {
	return &MockBlockchain{
		logWatchEnabled: true,
	}
}

// Protocol ...
func (m *MockBlockchain) Protocol() string {
	return "mock-protocol"
}

// IsConfigured ...
func (m *MockBlockchain) IsConfigured() bool {
	panic("not implemented")
}

// ResetConfig ...
func (m *MockBlockchain) ResetConfig() error {
	panic("not implemented")
}

// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
func (m *MockBlockchain) PEFEndpoints() []global.PEFEndpoint {
	panic("not implemented")
}

// ContainerRegex returns a regex-compatible strings to identify the blockchain node
// if it is running as a docker container.
func (m *MockBlockchain) ContainerRegex() []string {
	panic("not implemented")
}

// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
func (m *MockBlockchain) LogEventsList() map[string]model.FromContext {
	panic("not implemented")
}

// NodeLogPath returns the path to the log file to watch.
// Supports special keys like "docker" or "journald <service-name>"
func (m *MockBlockchain) NodeLogPath() string {
	panic("not implemented")
}

// NodeID ...
func (m *MockBlockchain) NodeID() string {
	return "mock-node-id"
}

// NodeRole ...
func (m *MockBlockchain) NodeRole() string {
	return "mock-node-type"
}

// NodeVersion ...
func (m *MockBlockchain) NodeVersion() string {
	return "mock-node-version"
}

// Network ...
func (m *MockBlockchain) Network() string {
	return "mock-node-network"
}

// LogWatchEnabled specifies if a node wants its logs to be read
func (m *MockBlockchain) LogWatchEnabled() bool {
	m.RLock()
	defer m.RUnlock()
	return m.logWatchEnabled
}

// SetLogWatchEnabled sets LogWatchEnable return value
func (m *MockBlockchain) SetLogWatchEnabled(val bool) {
	m.Lock()
	defer m.Unlock()
	m.logWatchEnabled = val
}

// SetRunScheme ...
func (m *MockBlockchain) SetRunScheme(s global.NodeRunScheme) {
}

// SetDockerContainer ...
func (m *MockBlockchain) SetDockerContainer(c *types.Container) {
}

// SetSystemdService ...
func (m *MockBlockchain) SetSystemdService(u *dbus.UnitStatus) {
}

// ReconfigureByDockerContainer ...
func (m *MockBlockchain) ReconfigureByDockerContainer(container *types.Container, reader io.ReadCloser) error {
	return nil
}

// ReconfigureBySystemdUnit ...
func (m *MockBlockchain) ReconfigureBySystemdUnit(unit *dbus.UnitStatus, reader io.ReadCloser) error {
	return nil
}

// ConfigUpdateCh ...
func (m *MockBlockchain) ConfigUpdateCh() chan global.ConfigUpdate {
	return nil
}

// PlatformEnabled enabled by default for Flow
func (m *MockBlockchain) PlatformEnabled() bool {
	return true
}

// DiscoveryDeactivated enabled by default for Flow
func (m *MockBlockchain) DiscoveryDeactivated() bool {
	return false
}

// RuntimeDisableFingerprintValidation disabled by default for Flow
func (m *MockBlockchain) RuntimeDisableFingerprintValidation() bool {
	return false
}

// RuntimeWatchersInflux default influx watcher configuration
func (m *MockBlockchain) RuntimeWatchersInflux() *global.WatchConfig {
	return nil
}
