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
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"
)

type mockDiscoverer struct {
	isRunning bool
	errors    bool
	errorsMu  *sync.RWMutex
	nilSvc    bool
}

func (m *mockDiscoverer) DockerContainer() *types.Container {
	panic("not implemented")
}

func (m *mockDiscoverer) SystemdService() *dbus.UnitStatus {
	if m.nilSvc {
		return nil
	}

	if m.isRunning {
		return &dbus.UnitStatus{SubState: "running"}
	}

	return &dbus.UnitStatus{SubState: "dead"}
}

func (m *mockDiscoverer) DetectScheme(ctx context.Context) global.NodeRunScheme {
	return global.NodeSystemd
}

func (m *mockDiscoverer) DetectDockerContainer(ctx context.Context) (*types.Container, error) {
	panic("not implemented")
}

func (m *mockDiscoverer) DetectSystemdService(ctx context.Context) (*dbus.UnitStatus, error) {
	m.errorsMu.RLock()
	if m.errors {
		defer m.errorsMu.RUnlock()
		return nil, errors.New("mock discoverer error")
	}
	defer m.errorsMu.RUnlock()

	if m.nilSvc {
		return nil, nil
	}

	if m.isRunning {
		return &dbus.UnitStatus{SubState: "running"}, nil
	}

	return &dbus.UnitStatus{SubState: "dead"}, nil
}

func (m *mockDiscoverer) setErrors(e bool) {
	m.errorsMu.Lock()
	defer m.errorsMu.Unlock()

	m.errors = e
}

func TestSystemdServiceWatchNew(t *testing.T) {
	tests := []struct {
		name                string
		isRunning           bool
		errors              bool
		nilSvc              bool
		initialRuntimeState int32
		expEv               string
		expNodeDownCnt      int
		expNodeUpCnt        int
	}{
		{
			name:      "success",
			isRunning: true,
			expEv:     model.AgentNodeUpName,
		},
		{
			name:                "success (already up)",
			isRunning:           true,
			initialRuntimeState: global.NodeDiscoverySuccess,
		},
		{
			name:  "service not running",
			expEv: model.AgentNodeDownName,
		},
		{
			name:                "service not running (already down)",
			initialRuntimeState: global.NodeDiscoveryError,
		},
		{
			name:   "nil service",
			nilSvc: true,
			expEv:  model.AgentNodeDownName,
		},
		{
			name:                "nil service (already down)",
			nilSvc:              true,
			initialRuntimeState: global.NodeDiscoveryError,
		},
	}

	defer global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			global.AgentRuntimeState.SetDiscoveryState(tt.initialRuntimeState)

			mockDsc := &mockDiscoverer{
				errors:    tt.errors,
				errorsMu:  &sync.RWMutex{},
				isRunning: tt.isRunning,
				nilSvc:    tt.nilSvc,
			}
			conf := SystemdServiceWatchConf{
				StatusIntv: time.Millisecond,
				Discoverer: mockDsc,
			}

			w, err := NewSystemdServiceWatch(conf)
			require.Nil(t, err)
			defer w.Stop()

			ch := make(chan interface{}, 10)
			w.Subscribe(ch)

			w.StartUnsafe()

			if tt.expEv != "" {
				select {
				case ev := <-ch:
					event, ok := ev.(*model.Message)
					require.True(t, ok)
					require.Equal(t, tt.expEv, event.Name)
				case <-time.After(5 * time.Second):
					t.Error("timeout waiting for event")
				}
			}
		})
	}
}

func TestSystemdServiceWatch_NodeRestart(t *testing.T) {
	mockDsc := &mockDiscoverer{
		errorsMu:  &sync.RWMutex{},
		isRunning: true,
	}
	conf := SystemdServiceWatchConf{
		StatusIntv: time.Millisecond,
		Discoverer: mockDsc,
	}

	w, err := NewSystemdServiceWatch(conf)
	require.Nil(t, err)
	defer w.Stop()

	ch := make(chan interface{}, 10)
	w.Subscribe(ch)

	w.StartUnsafe()

	ev := <-ch
	event, ok := ev.(*model.Message)
	require.True(t, ok)
	require.Equal(t, "agent.node.up", event.Name)

	mockDsc.setErrors(true)
	ev = <-ch
	event, ok = ev.(*model.Message)
	require.True(t, ok)
	require.Equal(t, "agent.node.down", event.Name)

	mockDsc.setErrors(false)
	ev = <-ch
	event, ok = ev.(*model.Message)
	require.True(t, ok)
	require.Equal(t, "agent.node.up", event.Name)
}
