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
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"agent/internal/pkg/cloudproviders"
	"agent/internal/pkg/cloudproviders/do"
	"agent/internal/pkg/cloudproviders/ec2"
	"agent/internal/pkg/cloudproviders/equinix"
	"agent/internal/pkg/cloudproviders/gce"
	"agent/internal/pkg/cloudproviders/vultr"

	"github.com/stretchr/testify/require"
)

func TestAgentPrepareStartup(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	t.Log(tmpdir)

	err := AgentPrepareStartup()
	require.Nil(t, err)

	expCacheDir := ".cache"
	files, err := ioutil.ReadDir(tmpdir)
	require.Nil(t, err)

	gotFiles := []string{}
	for _, file := range files {
		gotFiles = append(gotFiles, file.Name())
	}
	require.Lenf(t, files, 1, "expected only %s dir but got %v", expCacheDir, gotFiles)
	gotFile := files[0]
	require.True(t, gotFile.IsDir())

	require.Equal(t, expCacheDir, gotFiles[0])

	// now check .cache/ contents
	files, err = ioutil.ReadDir(filepath.Join(tmpdir, gotFile.Name()))
	require.Nil(t, err)

	gotFiles = []string{}
	for _, file := range files {
		gotFiles = append(gotFiles, file.Name())
	}

	require.Len(t, files, 1)
	require.Equal(t, "ma_fingerprint", files[0].Name())
}

func TestAgentPrepareStartup_FingerpintMismatch(t *testing.T) {
	tmpdir := t.TempDir()
	t.Setenv("HOME", tmpdir)
	t.Log(tmpdir)

	err := AgentPrepareStartup()
	require.Nil(t, err)

	// now rewrite cached fingerpint to play out the mismatch scenario
	fakeFingerpint := []byte("fingerprint_mismatch")
	fingerprintPath := filepath.Join(tmpdir, ".cache/ma_fingerprint")
	err = ioutil.WriteFile(fingerprintPath, fakeFingerpint, fs.ModePerm)
	require.Nil(t, err)

	err = AgentPrepareStartup()
	require.NotNil(t, err)
}

type MockCheck struct{}

// Name returns the providers name
func (m *MockCheck) Name() string {
	return "mock-provider"
}

// IsRunningOn returns true if the agent is running on a specific provider.
func (m *MockCheck) IsRunningOn() bool {
	time.Sleep(10 * time.Millisecond)

	return true
}

// Hostname returns the hostname as reported by the providers metadata remote store.
func (m *MockCheck) Hostname() (string, error) {
	time.Sleep(20 * time.Millisecond)

	return "mock-hostname", nil
}

func TestAgentSetHostname(t *testing.T) {
	providers := []cloudproviders.MetadataSearch{
		gce.NewSearch(),
		do.NewSearch(),
		equinix.NewSearch(),
		ec2.NewSearch(),
		vultr.NewSearch(),
		// azure.NewSearch(),
		&MockCheck{},
	}

	err := setAgentHostname(providers)
	require.Nil(t, err)
	require.Equal(t, "mock-hostname", AgentHostname)
}
