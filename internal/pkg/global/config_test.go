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
`)

	_, err = f.Write(testConf)
	require.NoError(t, err)

	configFilePriorityWas := ConfigFilePriority
	ConfigFilePriority = []string{f.Name()}
	defer func() { ConfigFilePriority = configFilePriorityWas }()

	err = os.Setenv("MA_API_KEY", "foobar")
	err = os.Setenv("MA_PLATFORM", "foobar.addr:443")

	err = LoadAgentConfig()
	require.NoError(t, err)

	require.Equal(t, "foobar", AgentConf.Platform.APIKey)
	require.Equal(t, "foobar.addr:443", AgentConf.Platform.Addr)
}
