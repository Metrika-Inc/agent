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
	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/docker/docker/api/types"
)

// MockBlockchain ...
type MockBlockchain struct{}

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

// NodeType ...
func (m *MockBlockchain) NodeType() string {
	return "mock-node-type"
}

// NodeVersion ...
func (m *MockBlockchain) NodeVersion() string {
	return "mock-node-version"
}

// DiscoverContainer returns the container discovered or an error if any occurs
func (m *MockBlockchain) DiscoverContainer() (*types.Container, error) {
	return &types.Container{Names: []string{"/flow-private-network_consensus_3_1"}}, nil
}

// Network ...
func (m *MockBlockchain) Network() string {
	return "mock-node-network"
}
