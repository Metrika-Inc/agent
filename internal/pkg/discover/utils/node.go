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

package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

var (
	// ErrNodeUndetected node was not found
	ErrNodeUndetected = errors.New("no node detected")

	// ErrNodeServiceNotFound systemd service not found
	ErrNodeServiceNotFound = errors.New("no node run by systemd detected")

	// ErrNodeDiscovererConfig node discoverer configuration error
	ErrNodeDiscovererConfig = errors.New("at least one regular expression is required (i.e. containerRegex)")

	// ErrNodeDiscoveryCancelled node discovery cancelled error
	ErrNodeDiscoveryCancelled = errors.New("node discovery cancelled")

	// supportedSchemes list of supported running schemes
	supportedSchemes = map[global.NodeRunScheme]bool{global.NodeDocker: true, global.NodeSystemd: true}
)

// NodeDiscovery describes an interface for discovery of various types of node running modes.
type NodeDiscovery interface {
	DockerContainer() *types.Container
	SystemdService() *dbus.UnitStatus
	DetectScheme(ctx context.Context) global.NodeRunScheme
	DetectDockerContainer(ctx context.Context) (*types.Container, error)
	DetectSystemdService(ctx context.Context) (*dbus.UnitStatus, error)
}

// NodeDiscovererConfig used to configure NodeDiscoverer
type NodeDiscovererConfig struct {
	ContainerRegex []string
	UnitGlob       []string
}

// NodeDiscoverer is the main implementation of the NodeDiscovery interface and provides
// a convenient way to access the node discovery results. It is not thread-safe.
type NodeDiscoverer struct {
	NodeDiscovererConfig

	container      *types.Container
	service        *dbus.UnitStatus
	dbusConn       *dbus.Conn
	dbusConnRepair bool
}

// NewNodeDiscoverer builds a new node discoverer object useful at agent startup
// and across node watch facilities.
func NewNodeDiscoverer(c NodeDiscovererConfig) (*NodeDiscoverer, error) {
	if len(c.UnitGlob) == 0 && len(c.ContainerRegex) == 0 {
		return nil, ErrNodeDiscovererConfig
	}

	return &NodeDiscoverer{NodeDiscovererConfig: c}, nil
}

// DockerContainer returns the detected docker container
func (n *NodeDiscoverer) DockerContainer() *types.Container {
	return n.container
}

// SystemdService returns the detected systemd unit object
func (n *NodeDiscoverer) SystemdService() *dbus.UnitStatus {
	return n.service
}

// DetectDockerContainer lists all containers and matches its names against a configured
// regular expression and updates the cached container object.
func (n *NodeDiscoverer) DetectDockerContainer(ctx context.Context) (*types.Container, error) {
	containers, err := GetRunningContainers()
	if err != nil {
		n.container = nil
		return nil, err
	}

	container, err := MatchContainer(containers, n.ContainerRegex)
	if err != nil {
		n.container = nil
		return nil, err
	}

	n.container = &container

	return &container, nil
}

// DetectSystemdService lists all systemd units and matches its names against a configured
// regular expression and updates the cached service object.
func (n *NodeDiscoverer) DetectSystemdService(ctx context.Context) (*dbus.UnitStatus, error) {
	if n.dbusConn == nil {
		conn, err := dbus.NewWithContext(ctx)
		if err != nil {
			return nil, err
		}
		n.dbusConn = conn
	}

	zap.S().Debugw("listing systemd units", "glob", n.UnitGlob)

	units, err := n.dbusConn.ListUnitsByPatternsContext(ctx, []string{"running"}, n.UnitGlob)
	if err != nil {
		n.service = nil
		n.dbusConn.Close()
		n.dbusConn = nil // repair connection on next call
		return nil, err
	}

	if units == nil || len(units) == 0 {
		return nil, ErrNodeServiceNotFound
	}

	if len(units) > 1 {
		zap.S().Warnw("more than systemd units found", "len", len(units))
	}

	n.service = &units[0]

	zap.S().Infow("successfully discovered systemd node", "glob", n.UnitGlob)

	return &units[0], nil
}

// Close releases underlying resources
func (n *NodeDiscoverer) Close() {
	if n.dbusConn != nil {
		n.dbusConn.Close()
	}
}

var tailLines = uint64(100)

// NewJournalReader returns an io.Reader to read journald logs for the discovered systemd unit.
func NewJournalReader(glob string) (io.ReadCloser, error) {
	formatter := func(entry *sdjournal.JournalEntry) (string, error) {
		v, ok := entry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE]
		if !ok {
			return "", fmt.Errorf("journal entry without SD_JOURNAL_FIELD_MESSAGE field")
		}
		return v + "\n", nil
	}

	jrc := sdjournal.JournalReaderConfig{}
	jrc.Formatter = formatter
	jrc.Matches = []sdjournal.Match{
		{Field: sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT, Value: glob},
	}
	jrc.NumFromTail = tailLines

	reader, err := sdjournal.NewJournalReader(jrc)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// NewDockerLogsReader returns an io.Reader to read docker logs of the discovered container.
func NewDockerLogsReader(name string) (io.ReadCloser, error) {
	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       fmt.Sprintf("%d", tailLines),
	}

	reader, err := DockerLogs(context.Background(), name, opts)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

func (n *NodeDiscoverer) detect(ctx context.Context) (global.NodeRunScheme, error) {
	res := make(chan error, len(supportedSchemes))
	log := zap.S()

	activeSchemes := 0
	if len(n.ContainerRegex) > 0 {
		activeSchemes++
		go func() {
			log.Debugw("starting docker container detection", "regex", n.ContainerRegex)
			container, err := n.DetectDockerContainer(context.TODO())
			if err != nil {
				if err == ErrContainerNotFound {
					log.Warnw("no docker container found", "containerRegex", n.ContainerRegex)
				} else {
					log.Debugw("error detecting docker container", zap.Error(err))
				}
				res <- err
				return
			}

			log.Infow("docker container detected", "names", container.Names, "status", container.Status)
			res <- nil
		}()
	}

	if len(n.UnitGlob) > 0 {
		activeSchemes++
		go func() {
			log.Debugw("starting systemd service detection", "regex", n.UnitGlob)
			service, err := n.DetectSystemdService(ctx)
			if err != nil {
				if err == ErrNodeServiceNotFound {
					log.Warnw("no systemd service found", "name", n.UnitGlob)
				} else {
					log.Debugw("error detecting systemd service", zap.Error(err))
				}
				res <- err
				return
			}

			log.Infow("systemd service detected", "name", service.Name, "active_state", service.ActiveState, "substate", service.SubState)
			res <- nil
		}()
	}

	for i := 0; i < activeSchemes; i++ {
		select {
		case <-ctx.Done():
			return -1, ErrNodeDiscoveryCancelled
		case err := <-res:
			if errors.Is(err, ErrNodeServiceNotFound) || errors.Is(err, ErrContainerNotFound) {
				continue
			}

			if err != nil {
				log.Debugw("one of the discovery routines failed", zap.Error(err))
			}
		}
	}

	// wait for discovery to finish and prioritize docker
	if n.container != nil {
		return global.NodeDocker, nil
	} else if n.service != nil {
		return global.NodeSystemd, nil
	} else {
		return -1, ErrNodeUndetected
	}
}

// DetectScheme blocks forever until node is discovered run by any of the supported schemes
// or if the passed context is cancelled. Returns the detected run scheme or -1 if not found.
func (n *NodeDiscoverer) DetectScheme(ctx context.Context) global.NodeRunScheme {
	if n.container != nil {
		return global.NodeDocker
	} else if n.service != nil {
		return global.NodeSystemd
	}

	for {
		scheme, err := n.detect(ctx)
		if err != nil && errors.Is(err, ErrNodeDiscoveryCancelled) {
			return -1
		}

		if err != nil {
			zap.S().Warnw("could not detect node, retrying in 2s", zap.Error(err))

			time.Sleep(2 * time.Second)
			continue
		}
		zap.S().Debugw("node scheme detected", "scheme", scheme)

		switch scheme {
		case global.NodeDocker:
			if n.dbusConn != nil {
				n.dbusConn.Close()
				n.dbusConn = nil
			}
			n.service = nil
		}

		if _, ok := supportedSchemes[scheme]; ok {
			return scheme
		}

		select {
		case <-ctx.Done():
			return -1
		default:
			time.Sleep(2 * time.Second)
		}
	}
}
