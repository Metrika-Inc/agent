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

type MockBlockchain struct{}

func (m *MockBlockchain) Protocol() string {
	return "mock-protocol"
}

func (m *MockBlockchain) IsConfigured() bool {
	panic("not implemented") // TODO: Implement
}

func (m *MockBlockchain) ResetConfig() error {
	panic("not implemented") // TODO: Implement
}

// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
func (m *MockBlockchain) PEFEndpoints() []global.PEFEndpoint {
	panic("not implemented") // TODO: Implement
}

// ContainerRegex returns a regex-compatible strings to identify the blockchain node
// if it is running as a docker container.
func (m *MockBlockchain) ContainerRegex() []string {
	panic("not implemented") // TODO: Implement
}

// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
func (m *MockBlockchain) LogEventsList() map[string]model.FromContext {
	panic("not implemented") // TODO: Implement
}

// NodeLogPath returns the path to the log file to watch.
// Supports special keys like "docker" or "journald <service-name>"
// TODO: string -> []string perhaps
func (m *MockBlockchain) NodeLogPath() string {
	panic("not implemented") // TODO: Implement
}

func (m *MockBlockchain) NodeID() string {
	return "mock-node-id"
}

func (m *MockBlockchain) NodeType() string {
	return "mock-node-type"
}

func (m *MockBlockchain) NodeVersion() string {
	return "mock-node-version"
}

// DiscoverContainer returns the container discovered or an error if any occurs
func (m *MockBlockchain) DiscoverContainer() (*types.Container, error) {
	return &types.Container{}, nil
}

func (m *MockBlockchain) Network() string {
	return "mock-node-network"
}
