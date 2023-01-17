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
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"strings"

	"github.com/pkg/errors"

	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

var (
	chain      global.Chain
	protocol   string
	configPath string
)

// ensureRequired ensures global agent configuration has loaded required configuration
func ensureRequired(c *global.AgentConfig) error {
	// Platform variables are only required if Platform Exporter is enabled
	if c.Platform.IsEnabled() {
		if c.Platform.Addr == "" || c.Platform.Addr == global.PlatformAddrConfigPlaceholder {
			return fmt.Errorf("platform.addr is missing from loaded config")
		}

		if c.Platform.APIKey == "" || c.Platform.APIKey == global.PlatformAPIKeyConfigPlaceholder {
			return fmt.Errorf("platform.api_key is missing from loaded config")
		}
	}

	return nil
}

// userLookup is interface to enable mocking os/user package.
type userLookup interface {
	Current() (*user.User, error)
	LookupGroupID(gid string) (*user.Group, error)
}

type osUserLookup struct {
	*user.User
}

func (o *osUserLookup) Current() (*user.User, error) {
	return user.Current()
}

func (o *osUserLookup) LookupGroupID(gid string) (*user.Group, error) {
	return user.LookupGroupId(gid)
}

var getUserGroupIds = func(u *user.User) ([]string, error) {
	return u.GroupIds()
}

// systemdCanBeActivated returns true if agent user is part of systemd-journal group. Returns false
// and the error if one occurs. Used to deactivate systemd node discovery path.
func systemdCanBeActivated(usr userLookup, targetGrp string) (bool, error) {
	u, err := usr.Current()
	if err != nil {
		return false, err
	}

	gids, err := getUserGroupIds(u)
	if err != nil {
		return false, err
	}

	for _, gid := range gids {
		grp, err := usr.LookupGroupID(gid)
		if err != nil {
			continue
		}
		if grp.Name == targetGrp {
			return true, nil
		}
	}

	return false, nil
}

func clearDeactivatedDiscoveryConfig(c *global.AgentConfig) {
	if c.Discovery.Docker.Deactivated {
		c.Discovery.Docker.Regex = []string{}
	}

	if c.Discovery.Systemd.Deactivated {
		c.Discovery.Systemd.Glob = []string{}
	}
}

// ensureSystemdDeactivated force sets discovery.systemd.deactivated to true
// if agent user is not part of systemd-journal group.
func ensureSystemdDeactivated(c *global.AgentConfig) error {
	usr := &osUserLookup{}

	act, err := systemdCanBeActivated(usr, "systemd-journal")
	if err != nil {
		return err
	}

	if !act {
		c.Discovery.Systemd.Deactivated = true
	}

	return nil
}

// AutoConfig initializes the blockchain node for the agent runtime.
func AutoConfig(c *global.AgentConfig, reset bool) (global.Chain, error) {
	Init()

	log := zap.S()
	if reset {
		if err := chain.ResetConfig(); err != nil {
			return nil, fmt.Errorf("failed to reset configuration: %v", err)
		}
	}

	chn, ok := chain.(global.Chain)
	if !ok {
		return nil, errors.New("blockchain package does not implement chain interface")
	}

	if chain.RuntimeDisableFingerprintValidation() {
		if !c.Runtime.DisableFingerprintValidation {
			log.Warnw("fingerprint validation disabled by default", "protocol", chain.Protocol())
		}
		c.Runtime.DisableFingerprintValidation = true
	}

	if chain.DiscoveryDeactivated() {
		if !c.Discovery.Deactivated {
			log.Warnw("node discovery disabled by default, ignoring discovery.deactivated config", "protocol", chain.Protocol())
		}

		c.Discovery.Deactivated = true
	} else {
		if len(c.Discovery.Systemd.Glob) == 0 {
			c.Discovery.Systemd.Glob = DefaultDiscoveryHintsSystemd
		}

		if len(c.Discovery.Docker.Regex) == 0 {
			c.Discovery.Docker.Regex = DefaultDiscoveryHintsDocker
		}

		if err := ensureSystemdDeactivated(c); err != nil {
			return nil, errors.Wrapf(err, "error checking systemd user group")
		}
	}

	clearDeactivatedDiscoveryConfig(c)

	// check if influx watcher has been explcitly configured in agent.yml
	hasInfluxConfigured := false
	for _, wc := range c.Runtime.Watchers {
		wt := global.WatchType(wc.Type)
		if wt.IsInflux() {
			hasInfluxConfigured = true
			break
		}
	}

	// if not configured in agent.yml ask the Chain interface if it must always be activated for that protocol
	if ixconf := chain.RuntimeWatchersInflux(); !hasInfluxConfigured && ixconf != nil {
		c.Runtime.Watchers = append(c.Runtime.Watchers, ixconf)
		for _, wc := range global.AgentConf.Runtime.Watchers {
			wt := global.WatchType(wc.Type)
			if wt.IsInflux() {
				wc.ExporterActivated = true
				v := os.Getenv(strings.ToUpper(global.ConfigEnvPrefix + "_" + "runtime_watchers_influx_upstream_url"))
				if v != "" {
					wc.UpstreamURL = v
				}
				v = os.Getenv(strings.ToUpper(global.ConfigEnvPrefix + "_" + "runtime_watchers_influx_listen_addr"))
				if v != "" {
					wc.ListenAddr = v
				}
			}
		}
	}

	if !chain.PlatformEnabled() {
		if *c.Platform.Enabled {
			log.Warnw("platform exporter is deactivated by default, ignoring platform.enabled config", "protocol", chain.Protocol())
		}
		*c.Platform.Enabled = false
	} else {
		if err := ensureRequired(c); err != nil {
			return nil, err
		}
	}

	if ok := chain.IsConfigured(); ok {
		log.Info("protocol configuration OK")

		return chn, nil
	}

	return chn, nil
}

// ResetConfig removes the protocol's configuration files.
// Allows discovery process to begin anew.
func ResetConfig() {
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			zap.S().Errorw("failed to remove a config file", zap.Error(err))
		}
	}
}
