// Copyright 2015 The Prometheus Authors
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

package collector

import (
	"flag"
	"strings"
	"testing"

	"github.com/prometheus/procfs"
	"github.com/prometheus/procfs/sysfs"
	"github.com/stretchr/testify/require"
)

func TestDefaultProcPath(t *testing.T) {
	if got, want := procFilePath("somefile"), "/proc/somefile"; got != want {
		t.Errorf("Expected: %s, Got: %s", want, got)
	}

	if got, want := procFilePath("some/file"), "/proc/some/file"; got != want {
		t.Errorf("Expected: %s, Got: %s", want, got)
	}
}

func TestDefaultSysPath(t *testing.T) {
	if got, want := sysFilePath("somefile"), "/sys/somefile"; got != want {
		t.Errorf("Expected: %s, Got: %s", want, got)
	}

	if got, want := sysFilePath("some/file"), "/sys/some/file"; got != want {
		t.Errorf("Expected: %s, Got: %s", want, got)
	}
}

func TestOverrideFSPaths(t *testing.T) {
	tests := []struct {
		flags     *flag.FlagSet
		args      []string
		expProcFs string
		expSysFs  string
		expRootFs string
	}{
		{
			flag.NewFlagSet("metrikad", flag.ContinueOnError),
			[]string{},
			procfs.DefaultMountPoint,
			sysfs.DefaultMountPoint,
			"/",
		},
		{
			flag.NewFlagSet("metrikad", flag.ContinueOnError),
			[]string{"-procfs", "/host/proc", "-sysfs", "/host/sys", "-rootfs", "/host"},
			"/host/proc",
			"/host/sys",
			"/host",
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			DefineFsPathFlags(tt.flags)
			err := tt.flags.Parse(tt.args)
			require.Nil(t, err)

			require.Equal(t, tt.expProcFs, procPath)
			require.Equal(t, tt.expSysFs, sysPath)
			require.Equal(t, tt.expRootFs, rootfsPath)
		})
	}
}
