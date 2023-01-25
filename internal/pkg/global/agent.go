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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"

	"agent/api/v1/model"
	"agent/internal/pkg/cloudproviders"
	"agent/internal/pkg/cloudproviders/azure"
	"agent/internal/pkg/cloudproviders/do"
	"agent/internal/pkg/cloudproviders/ec2"
	"agent/internal/pkg/cloudproviders/equinix"
	"agent/internal/pkg/cloudproviders/gce"
	"agent/internal/pkg/cloudproviders/otc"
	"agent/internal/pkg/cloudproviders/vultr"
	"agent/internal/pkg/fingerprint"

	"go.uber.org/zap"

	"github.com/docker/docker/api/types"
)

var (
	// BlockchainNode global used by agent to bind
	// implementations of the Chain interface (i.e. flow package)
	BlockchainNode Chain

	// Modified at runtime

	// Version agent version
	Version = "v0.0.0"

	// CommitHash commit hash computed at build time.
	CommitHash = ""
)

const (
	// cloudProviderDiscoveryTimeout max time to wait until at least
	// one provider metadata sever responds.
	cloudProviderDiscoveryTimeout = 1 * time.Second
)

// Chain provides necessary configuration information
// for the agent core. These methods represent currently
// supported sampler configurations per blockchain protocol.
type Chain interface {
	IsConfigured() bool
	ResetConfig() error

	// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
	PEFEndpoints() []PEFEndpoint

	// ContainerRegex returns a regex-compatible strings to identify the blockchain node
	// if it is running as a docker container.
	ContainerRegex() []string

	// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
	LogEventsList() map[string]model.FromContext

	// NodeLogPath returns the path to the log file to watch.
	// Supports special keys like "docker" or "journald <service-name>"
	NodeLogPath() string

	// NodeID returns the blockchain node id
	NodeID() string

	// NodeRole returns the blockchain node type (i.e. consensus)
	NodeRole() string

	// NodeVersion returns the blockchain node version
	NodeVersion() string

	// Protocol protocol name to use for the platform
	Protocol() string

	// Network network name the blockchain node is running on
	Network() string

	// LogWatchEnabled specifies if the specific the logs of
	// a specific node need to be watched or not.
	LogWatchEnabled() bool

	ReconfigureByDockerContainer(container *types.Container, reader io.ReadCloser) error
	ReconfigureBySystemdUnit(unit *dbus.UnitStatus, reader io.ReadCloser) error
	SetRunScheme(NodeRunScheme)
	SetDockerContainer(*types.Container)
	SetSystemdService(*dbus.UnitStatus)
}

// PEFEndpoint is a configuration for a single HTTP endpoint
// that exposes metrics in Prometheus Exposition Format.
type PEFEndpoint struct {
	URL     string   `json:"url" yaml:"URL"`
	Filters []string `json:"filters" yaml:"filters"`
}

// NodeRunScheme describes how the node process is managed on the host system
type NodeRunScheme int

const (
	// NodeDocker blockchain node is run by docker
	NodeDocker NodeRunScheme = 1

	// NodeSystemd blockchain node is run by systemd
	NodeSystemd NodeRunScheme = 2
)

// ErrNodeRunSchemeNotSet error used when the node run scheme is required for operational reasons
var ErrNodeRunSchemeNotSet = errors.New("node run scheme has not been set")

// NewFingerprintWriter opens a file for writing fingerprint values.
func NewFingerprintWriter(path string) *os.File {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		zap.S().Fatalw("failed opening a fingerprint file for writing", zap.Error(err))
	}

	return file
}

// NewFingerprintReader returns a ReadCloser
func NewFingerprintReader(path string) io.ReadCloser {
	file, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	if err != nil {
		zap.S().Fatalw("failed opening fingerprint file for reading", zap.Error(err))
	}

	return file
}

// FingerprintSetup sets up a new fingerpint and validates it against
// cached fingerpint, if any. If a fingerpint has not been previously
// cached (or removed by the user), writes the fingerpint to disk under
// the user's cache directory.
func FingerprintSetup() (string, error) {
	_, err := os.Stat(AgentCacheDir)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	if errors.Is(err, fs.ErrNotExist) {
		zap.S().Info("intializing cache directory: %s", AgentCacheDir)

		if err := os.MkdirAll(AgentCacheDir, 0o755); err != nil {
			return "", err
		}
	}

	fpp := filepath.Join(AgentCacheDir, DefaultFingerprintFilename)
	fpw := NewFingerprintWriter(fpp)
	defer fpw.Close()

	fpr := NewFingerprintReader(fpp)
	defer fpr.Close()

	fp, err := fingerprint.NewWithValidation([]byte(AgentHostname), fpw, fpr)
	if err != nil {
		if _, ok := err.(*fingerprint.ValidationError); ok {
			return "", fmt.Errorf("cached [%s]: %w", fpp, err)
		}
		return "", err
	}

	if err := fp.Write(); err != nil {
		return "", err
	}

	zap.S().Info("fingerprint ", fp.Hash())

	return fp.Hash(), nil
}

func setAgentHostname(providers []cloudproviders.MetadataSearch) error {
	var err error

	hostnameCh := make(chan string, len(providers))

	for _, provider := range providers {
		go func(provider cloudproviders.MetadataSearch) {
			if provider.IsRunningOn() {
				hostname, err := provider.Hostname()
				if err != nil {
					zap.S().Debugw("error getting hostname", "provider", provider.Name(), zap.Error(err))
					return
				}

				if hostname != "" {
					zap.S().Debugw("hostname found", "provider", provider.Name(), "hostname", hostname)
					hostnameCh <- hostname
				}
			}
		}(provider)
	}

	select {
	case AgentHostname = <-hostnameCh:
		if len(AgentHostname) == 0 {
			return fmt.Errorf("got empty hostname")
		}
	case <-time.After(cloudProviderDiscoveryTimeout):
		AgentHostname, err = os.Hostname()
		if err != nil {
			return errors.Wrapf(err, "could not get hostname from OS")
		}
	}

	return err
}

// AgentPrepareStartup sets up cache directory, agent hostname and fingerpint.
func AgentPrepareStartup() error {
	var err error

	// Agent cache directory (i.e $HOME/.cache/metrikad)
	AgentCacheDir, err = os.UserCacheDir()
	if err != nil {
		return errors.Wrapf(err, "user cache directory error: %v", err)
	}

	if err := os.Mkdir(AgentCacheDir, 0o755); err != nil &&
		!errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrExist) {

		return errors.Wrapf(err, "error creating cache directory: %s", AgentCacheDir)
	}

	// Set the agent hostname by one of the supported providers
	providers := []cloudproviders.MetadataSearch{
		gce.NewSearch(),
		do.NewSearch(),
		equinix.NewSearch(),
		ec2.NewSearch(),
		vultr.NewSearch(),
		azure.NewSearch(),
		otc.NewSearch(),
	}
	if err := setAgentHostname(providers); err != nil {
		return errors.Wrap(err, "error setting agent hostname")
	}

	// Fingerprint validation and caching persisted in the cache directory
	_, err = FingerprintSetup()
	if err != nil {
		if !AgentConf.Runtime.DisableFingerprintValidation {
			return errors.Wrap(err, "fingerprint initialization error")
		}
	}

	return nil
}
