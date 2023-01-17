package discover

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"testing"

	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
)

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

	configFilePriorityWas := global.ConfigFilePriority
	global.ConfigFilePriority = []string{f.Name()}
	defer func() { global.ConfigFilePriority = configFilePriorityWas }()

	c := &global.AgentConfig{}
	err = global.LoadAgentConfig(c)
	require.NoError(t, err)

	clearDeactivatedDiscoveryConfig(c)

	require.Empty(t, c.Discovery.Docker.Regex)
	require.Empty(t, c.Discovery.Systemd.Glob)
	require.True(t, c.Discovery.Docker.Deactivated)
	require.True(t, c.Discovery.Systemd.Deactivated)
}
