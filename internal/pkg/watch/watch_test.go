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

package watch

import (
	"io"
	"testing"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"
)

type mockBlockchain struct {
	nodeID      string
	nodeRole    string
	nodeVersion string
	network     string
}

func (m *mockBlockchain) IsConfigured() bool {
	panic("not implemented")
}

func (m *mockBlockchain) ResetConfig() error {
	panic("not implemented")
}

// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
func (m *mockBlockchain) PEFEndpoints() []global.PEFEndpoint {
	panic("not implemented")
}

// ContainerRegex returns a regex-compatible strings to identify the blockchain node
// if it is running as a docker container.
func (m *mockBlockchain) ContainerRegex() []string {
	panic("not implemented")
}

// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
func (m *mockBlockchain) LogEventsList() map[string]model.FromContext {
	panic("not implemented")
}

// NodeLogPath returns the path to the log file to watch.
// Supports special keys like "docker" or "journald <service-name>"
func (m *mockBlockchain) NodeLogPath() string {
	panic("not implemented")
}

// NodeID returns the blockchain node id
func (m *mockBlockchain) NodeID() string {
	return m.nodeID
}

// NodeRole returns the blockchain node type (i.e. consensus)
func (m *mockBlockchain) NodeRole() string {
	return m.nodeRole
}

// NodeVersion returns the blockchain node version
func (m *mockBlockchain) NodeVersion() string {
	return m.nodeVersion
}

// Protocol protocol name to use for the platform
func (m *mockBlockchain) Protocol() string {
	return "mock-chain"
}

// Network network name the blockchain node is running on
func (m *mockBlockchain) Network() string {
	return m.network
}

// LogWatchEnabled specifies if the specific the logs of
// a specific node need to be watched or not.
func (m *mockBlockchain) LogWatchEnabled() bool {
	panic("not implemented")
}

// ReconfigureByDockerContainer ...
func (m *mockBlockchain) ReconfigureByDockerContainer(container *types.Container, reader io.ReadCloser) error {
	return nil
}

// ReconfigureBySystemdUnit ...
func (m *mockBlockchain) ReconfigureBySystemdUnit(unit *dbus.UnitStatus, reader io.ReadCloser) error {
	return nil
}

func (m *mockBlockchain) SetRunScheme(_ global.NodeRunScheme) {
}

func (m *mockBlockchain) SetDockerContainer(_ *types.Container) {
}

func (m *mockBlockchain) SetSystemdService(_ *dbus.UnitStatus) {
}

func TestWatch_EmitAgentNodeEvents(t *testing.T) {
	tests := []struct {
		name        string
		nodeID      string
		nodeType    string
		nodeVersion string
		network     string
		expEvCtx    map[string]interface{}
	}{
		{
			name:        "all keys",
			nodeID:      "mock-node-id",
			nodeType:    "mock-node-type",
			nodeVersion: "mock-node-version",
			network:     "mock-node-network",
		},
		{
			name:        "missing node id",
			nodeType:    "mock-node-type",
			nodeVersion: "mock-node-version",
			network:     "mock-node-network",
		},
		{
			name:        "missing node type",
			nodeID:      "mock-node-id",
			nodeVersion: "mock-node-version",
			network:     "mock-node-network",
		},
		{
			name:     "missing node version",
			nodeID:   "mock-node-id",
			nodeType: "mock-node-type",
			network:  "mock-node-network",
		},
		{
			name:        "missing node network",
			nodeID:      "mock-node-id",
			nodeType:    "mock-node-type",
			nodeVersion: "mock-node-version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockchainNodeWas := global.BlockchainNode()
			global.SetBlockchainNode(&mockBlockchain{
				nodeID:      tt.nodeID,
				nodeRole:    tt.nodeType,
				nodeVersion: tt.nodeVersion,
				network:     tt.network,
			})
			defer func() { global.SetBlockchainNode(blockchainNodeWas) }()

			ch := make(chan interface{}, 10)
			w := NewWatch()
			w.Subscribe(ch)
			w.emitAgentNodeEvent(model.AgentNodeUpName)
			ev := <-ch

			msg, ok := ev.(*model.Message)
			require.True(t, ok)
			require.Equal(t, "agent.node.up", msg.Name)

			gotEv := msg.GetEvent()
			require.NotNil(t, gotEv)
			require.Equal(t, "agent.node.up", gotEv.Name)

			gotCtx := gotEv.Values.AsMap()
			if tt.nodeID == "" {
				require.NotContains(t, gotCtx, model.NodeIDKey)
			} else {
				require.Equal(t, tt.nodeID, gotEv.Values.AsMap()[model.NodeIDKey])
			}

			if tt.nodeType == "" {
				require.NotContains(t, gotCtx, model.NodeTypeKey)
			} else {
				require.Equal(t, tt.nodeType, gotEv.Values.AsMap()[model.NodeTypeKey])
			}

			if tt.nodeVersion == "" {
				require.NotContains(t, gotCtx, model.NodeVersionKey)
			} else {
				require.Equal(t, tt.nodeVersion, gotEv.Values.AsMap()[model.NodeVersionKey])
			}

			if tt.network == "" {
				require.NotContains(t, gotCtx, model.NetworkKey)
			} else {
				require.Equal(t, tt.network, gotEv.Values.AsMap()[model.NetworkKey])
			}
		})
	}
}
