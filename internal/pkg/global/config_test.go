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

package global

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCreateLogFolders(t *testing.T) {
	testCases := []struct {
		paths []string
	}{
		{[]string{"/tmp/metrikad/randomfile", "relativeFolder/randomfile"}},
	}

	for _, tc := range testCases {
		c := &AgentConfig{}
		c.Runtime.Log.Outputs = tc.paths
		err := createLogFolders(c)
		require.NoError(t, err)
		for _, path := range c.Runtime.Log.Outputs {
			_, err := os.Create(path)
			require.NoError(t, err)
			defer func() {
				pathSplit := strings.Split(path, "/")
				if len(pathSplit) == 1 {
					os.Remove(path)
				} else {
					os.RemoveAll(strings.Join(pathSplit[:len(pathSplit)-1], "/"))
				}
			}()
			_, err = os.Stat(path)
			require.NoError(t, err)
		}
	}
}

type mockUserLookup struct {
	currentErrors bool
	group         string
}

func (m *mockUserLookup) Current() (*user.User, error) {
	if m.currentErrors {
		return nil, errors.New("mock user.Current() error")
	}

	return &user.User{Name: "foobar"}, nil
}

func (m *mockUserLookup) LookupGroupID(gid string) (*user.Group, error) {
	if m.group == "" {
		return nil, fmt.Errorf("mock lookup group id")
	}

	return &user.Group{Name: m.group}, nil
}

func TestSystemdCanBeActivated(t *testing.T) {
	tests := []struct {
		name            string
		usr             userLookup
		usrGroupIdsFunc func(u *user.User) ([]string, error)
		expErr          bool
		expRes          bool
	}{
		{
			name: "can be activated",
			usr:  &mockUserLookup{group: "systemd-journal"},
			usrGroupIdsFunc: func(u *user.User) ([]string, error) {
				return []string{"1001"}, nil
			},
			expRes: true,
		},
		{
			name: "cannot be activated",
			usr:  &mockUserLookup{},
			usrGroupIdsFunc: func(u *user.User) ([]string, error) {
				return []string{"1001"}, nil
			},
			expRes: false,
		},
		{
			name: "with errors",
			usr:  &mockUserLookup{},
			usrGroupIdsFunc: func(u *user.User) ([]string, error) {
				return nil, errors.New("u.GroupIds() mock error")
			},
			expErr: true,
			expRes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getUserGroupIdsWas := getUserGroupIds
			getUserGroupIds = tt.usrGroupIdsFunc

			got, err := systemdCanBeActivated(tt.usr, "systemd-journal")
			if !tt.expErr {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
			}

			require.Equal(t, tt.expRes, got)
			getUserGroupIds = getUserGroupIdsWas
		})
	}
}

func TestClearDeactivatedDiscoveryConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	testConf := []byte(`
---
platform:
  api_key: test-api-key
  addr: test-addr
discovery:
  systemd:
    deactivated: true
    glob:
      - metrikad-*.service
  docker:
    deactivated: true
    regex:
      - metrikad-foobar
`)

	_, err = f.Write(testConf)
	require.NoError(t, err)

	configFilePriorityWas := ConfigFilePriority
	ConfigFilePriority = []string{f.Name()}
	defer func() { ConfigFilePriority = configFilePriorityWas }()

	c := &AgentConfig{}
	err = LoadAgentConfig(c)
	require.NoError(t, err)

	require.Empty(t, c.Discovery.Docker.Regex)
	require.Empty(t, c.Discovery.Systemd.Glob)
	require.True(t, c.Discovery.Docker.Deactivated)
	require.True(t, c.Discovery.Systemd.Deactivated)
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	testConf := []byte(`
---
platform:
  api_key: <api_key>
  addr: <platform_addr>
discovery:
  systemd:
    glob:
      - metrikad-*.service
`)

	_, err = f.Write(testConf)
	require.NoError(t, err)

	configFilePriorityWas := ConfigFilePriority
	ConfigFilePriority = []string{f.Name()}
	defer func() { ConfigFilePriority = configFilePriorityWas }()

	err = os.Setenv("MA_API_KEY", "foobar")
	err = os.Setenv("MA_PLATFORM_ENABLED", "false")
	err = os.Setenv("MA_PLATFORM", "foobar.addr:443")
	err = os.Setenv("MA_PLATFORM_BATCH_N", "100")
	err = os.Setenv("MA_PLATFORM_MAX_PUBLISH_INTERVAL", "1s")
	err = os.Setenv("MA_PLATFORM_TRANSPORT_TIMEOUT", "2s")
	err = os.Setenv("MA_PLATFORM_URI", "/")
	err = os.Setenv("MA_BUFFER_MAX_HEAP_ALLOC", "10000")
	err = os.Setenv("MA_BUFFER_MIN_BUFFER_SIZE", "100")
	err = os.Setenv("MA_BUFFER_TTL", "5s")
	err = os.Setenv("MA_RUNTIME_LOGGING_OUTPUTS", "stdout,stderr")
	err = os.Setenv("MA_RUNTIME_LOGGING_LEVEL", "debug")
	err = os.Setenv("MA_RUNTIME_DISABLE_FINGERPRINT_VALIDATION", "true")
	err = os.Setenv("MA_RUNTIME_HTTP_ADDR", "foobar:9000")
	err = os.Setenv("MA_RUNTIME_HOST_HEADER_VALIDATION_ENABLED", "false")
	err = os.Setenv("MA_RUNTIME_SAMPLING_INTERVAL", "5s")
	err = os.Setenv("MA_RUNTIME_WATCHERS", "foo,bar")
	err = os.Setenv("MA_DISCOVERY_DOCKER_REGEX", "container-name,foobar")
	err = os.Setenv("MA_DISCOVERY_SYSTEMD_GLOB", "node.service foobar")

	c := &AgentConfig{}
	err = LoadAgentConfig(c)
	require.NoError(t, err)

	require.Equal(t, "foobar", c.Platform.APIKey)
	require.Equal(t, false, *c.Platform.Enabled)
	require.Equal(t, "foobar.addr:443", c.Platform.Addr)
	require.Equal(t, 100, c.Platform.BatchN)
	require.Equal(t, 1*time.Second, c.Platform.MaxPublishInterval)
	require.Equal(t, 2*time.Second, c.Platform.TransportTimeout)
	require.Equal(t, "/", c.Platform.URI)
	require.Equal(t, uint64(10000), c.Buffer.MaxHeapAlloc)
	require.Equal(t, 100, c.Buffer.MinBufferSize)
	require.Equal(t, 5*time.Second, c.Buffer.TTL)
	require.Equal(t, []string{"stdout", "stderr"}, c.Runtime.Log.Outputs)
	require.Equal(t, "debug", c.Runtime.Log.Lvl)
	require.Equal(t, true, c.Runtime.DisableFingerprintValidation)
	require.Equal(t, "foobar:9000", c.Runtime.HTTPAddr)
	require.Equal(t, 5*time.Second, c.Runtime.SamplingInterval)
	require.Equal(t, false, *c.Runtime.HostHeaderValidationEnabled)
	require.Equal(t, []*WatchConfig{{Type: "foo", SamplingInterval: 5 * time.Second}, {Type: "bar", SamplingInterval: 5 * time.Second}}, c.Runtime.Watchers)
	require.Equal(t, "container-name", c.Discovery.Docker.Regex[0])
	require.Equal(t, "foobar", c.Discovery.Docker.Regex[1])
	require.Equal(t, "node.service", c.Discovery.Systemd.Glob[0])
	require.Equal(t, "foobar", c.Discovery.Systemd.Glob[1])
}
