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
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateLogFolders(t *testing.T) {
	testCases := []struct {
		paths []string
	}{
		{[]string{"/tmp/metrikad/randomfile", "relativeFolder/randomfile"}},
	}

	for _, tc := range testCases {
		AgentConf.Runtime.Log.Outputs = tc.paths
		err := createLogFolders()
		require.NoError(t, err)
		for _, path := range AgentConf.Runtime.Log.Outputs {
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

	err = LoadAgentConfig()
	require.NoError(t, err)

	require.Equal(t, "foobar", AgentConf.Platform.APIKey)
	require.Equal(t, false, *AgentConf.Platform.Enabled)
	require.Equal(t, "foobar.addr:443", AgentConf.Platform.Addr)
	require.Equal(t, 100, AgentConf.Platform.BatchN)
	require.Equal(t, 1*time.Second, AgentConf.Platform.MaxPublishInterval)
	require.Equal(t, 2*time.Second, AgentConf.Platform.TransportTimeout)
	require.Equal(t, "/", AgentConf.Platform.URI)
	require.Equal(t, uint64(10000), AgentConf.Buffer.MaxHeapAlloc)
	require.Equal(t, 100, AgentConf.Buffer.MinBufferSize)
	require.Equal(t, 5*time.Second, AgentConf.Buffer.TTL)
	require.Equal(t, []string{"stdout", "stderr"}, AgentConf.Runtime.Log.Outputs)
	require.Equal(t, "debug", AgentConf.Runtime.Log.Lvl)
	require.Equal(t, true, AgentConf.Runtime.DisableFingerprintValidation)
	require.Equal(t, "foobar:9000", AgentConf.Runtime.HTTPAddr)
	require.Equal(t, 5*time.Second, AgentConf.Runtime.SamplingInterval)
	require.Equal(t, false, *AgentConf.Runtime.HostHeaderValidationEnabled)
	require.Equal(t, []*WatchConfig{{Type: "foo", SamplingInterval: 5 * time.Second}, {Type: "bar", SamplingInterval: 5 * time.Second}}, AgentConf.Runtime.Watchers)
	require.Equal(t, "container-name", AgentConf.Discovery.Docker.Regex[0])
	require.Equal(t, "foobar", AgentConf.Discovery.Docker.Regex[1])
	require.Equal(t, "node.service", AgentConf.Discovery.Systemd.Glob[0])
	require.Equal(t, "foobar", AgentConf.Discovery.Systemd.Glob[1])
}
