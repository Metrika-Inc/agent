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

package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const (
	FlowVersionKey        = "FLOW_GO_NODE_VERSION"
	FlowNodeIDKey         = "FLOW_GO_NODE_ID"
	FlowExecutionNodeKey  = "FLOW_NETWORK_EXECUTION_NODE"
	FlowCollectionNodeKey = "FLOW_NETWORK_COLLECTION_NODE"

	// protocolName blockchain protocol name
	protocolName = "flow"

	// tailLinesStr is the default number of lines to fetch
	// when peeking the logfile for network/chain or node role data.
	tailLinesStr = "100"
)

// Flow is responsible for discovery and validation
// of the agent's flow-related configuration.
type Flow struct {
	config       FlowConfig
	renderNeeded bool // if any config value was empty but got updated
	container    *types.Container
	env          map[string]string
	nodeRole     string
	network      string
	nodeVersion  string
	mutex        *sync.RWMutex
}

func NewFlow() (*Flow, error) {
	flow := &Flow{mutex: &sync.RWMutex{}}
	config := NewFlowConfig(DefaultFlowPath)
	var err error
	flow.config, err = config.Load()
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		// configuration does not exist, create it
		flow.config, err = config.Default()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return flow, nil
}

func (d *Flow) ResetConfig() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var err error
	config := NewFlowConfig(DefaultFlowPath)
	d.config, err = config.Default()
	return err
}

func (d *Flow) IsConfigured() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.config.Client != "" && d.config.NodeID != "" && d.isPEFConfigured() {
		zap.S().Debug("protocol is already configured, nothing to do here")
		return true
	}
	return false
}

func (d *Flow) isPEFConfigured() bool {
	if len(d.config.PEFEndpoints) == 0 {
		zap.S().Fatal("pefEndpoints field should always have an entry; running agent with reset flag should populate it")
	}

	return len(d.config.PEFEndpoints[0].URL) != 0
}

func (d *Flow) DiscoverContainer() (*types.Container, error) {
	log := zap.S()
	log.Info("flow not fully configured, starting discovery")

	env, err := utils.GetEnvFromFile(d.config.EnvFilePath)
	if err != nil {
		log.Warnw("failed to load environment file", zap.Error(err))
	} else {
		// we need an else block because env gets initialized and returned
		// even if an error is encountered
		d.env = env
	}

	errs := &utils.AutoConfigError{}
	containers, err := utils.GetRunningContainers()
	if err != nil {
		log.Warnw("cannot access docker daemon, will attempt to auto-configure anyway", zap.Error(err))
	} else {
		container, err := utils.MatchContainer(containers, d.config.ContainerRegex)
		if err != nil {
			log.Warnw("unable to find running flow-go docker container, will attempt to auto-configure anyway")
			errs.Append(err)
		} else {
			log.Infow("discovered container with names", "names", container.Names)
			d.mutex.Lock()
			d.container = &container
			d.mutex.Unlock()
		}
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.configureClient(); err != nil {
		log.Warn("could not find client name")

		errs.Append(err)
	}
	if _, err := d.configureNodeID(); err != nil {
		log.Warn("could not find node ID")
		errs.Append(err)
	}

	if d.container != nil && len(d.container.Names) > 0 {
		if err := d.updateFromLogs(d.container.Names[0]); err != nil {
			log.Warnw("error while getting node metadata from logs", zap.Error(err))
			errs.Append(err)
		}
	}

	if _, err := d.updateNodeVersion(); err != nil {
		log.Warn("could not find node version")
		// errs.Append(err)
	}

	if err := d.configurePEFEndpoints(); err != nil {
		log.Warn("could not find PEF metric endpoints")
		errs.Append(err)
	}

	if d.renderNeeded {
		if err := global.GenerateConfigFromTemplate(DefaultTemplatePath,
			DefaultFlowPath, d.config); err != nil {
			log.Errorw("failed to generate the template", zap.Error(err))
			errs.Append(err)
		}
		d.renderNeeded = false
		FlowConf = &d.config
	}

	return d.container, errs.ErrIfAny()
}

func (d *Flow) NodeID() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.NodeID
}

func (d *Flow) configureNodeID() (string, error) {
	log := zap.S()
	if d.config.NodeID != "" {
		log.Debugw("NodeID exists, skipping discovery", "node_id", d.config.NodeID)
		return d.config.NodeID, nil
	}
	if d.container != nil {
		args := strings.Split(d.container.Command, " ")
		var nodeID string
		for i := 0; i < len(args); i++ {
			if strings.HasPrefix(args[i], "--nodeid") {
				if eqIndex := strings.Index(args[i], "="); eqIndex != -1 {
					nodeID = args[i][eqIndex+1:]
				} else {
					if i+1 >= len(args) {
						return "", fmt.Errorf("potentially invalid docker run command: %s", d.container.Command)
					}
					nodeID = args[i+1]
				}
				if nodeID != "" {
					log.Infow("node id found", "node_id", nodeID)
					d.config.NodeID = nodeID
					d.renderNeeded = true
					return d.config.NodeID, nil
				}
			}
		}
	}

	// fall back to environment file
	if d.env != nil {
		if nodeID, ok := d.env[FlowNodeIDKey]; ok {
			log.Infow("node id found", "node_id", nodeID)
			d.config.NodeID = nodeID
			d.renderNeeded = true
			return d.config.NodeID, nil
		}
	}
	return "", errors.New("node ID not found")
}

func (d *Flow) configurePEFEndpoints() error {
	if d.isPEFConfigured() {
		zap.S().Debug("PEFEndpoints already exist, skipping discovery")
		return nil
	}
	var found bool
	portsToTry := map[int]struct{}{
		8080: {},
		9095: {},
		9096: {},
	}
	if d.container != nil {
		for _, port := range d.container.Ports {
			if port.PublicPort != 3569 && port.PublicPort != 9000 {
				portsToTry[int(port.PublicPort)] = struct{}{}
			}
		}
	}

	cl := http.Client{
		Timeout: 500 * time.Millisecond,
	}

	// MetricEndpoints[0].Filters is hardcoded in template
	defaultFilters := d.config.PEFEndpoints[0].Filters

	for port := range portsToTry {
		endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/metrics"

		resp, err := cl.Get(endpoint)
		if err != nil {
			continue
		}
		if resp.StatusCode <= 204 {
			zap.S().Infow("found PEF metrics", "endpoint", endpoint)
			// MetricEndpoints[0].URL is hardcoded as "" in template
			if d.config.PEFEndpoints[0].URL == "" {
				d.config.PEFEndpoints[0].URL = endpoint
			} else {
				PEFEndpoint := global.PEFEndpoint{
					URL:     endpoint,
					Filters: defaultFilters,
				}
				d.config.PEFEndpoints = append(d.config.PEFEndpoints, PEFEndpoint)
			}
			found = true
			d.renderNeeded = true
		}
	}

	if !found {
		return errors.New("no PEF endpoints found")
	}
	return nil
}

func (d *Flow) ValidateClient() error {
	// flow-go is the only flow client
	if d.config.Client != "flow-go" {
		return errors.New("invalid client specified")
	}
	return nil
}

func (d *Flow) configureClient() error {
	if d.config.Client == "" {
		d.config.Client = "flow-go"
	}
	return nil
}

func (d *Flow) PEFEndpoints() []global.PEFEndpoint {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.PEFEndpoints
}

func (d *Flow) ContainerRegex() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.ContainerRegex
}

func (d *Flow) LogEventsList() map[string]model.FromContext {
	return eventsFromContext
}

func (d *Flow) NodeLogPath() string {
	return ""
}

func (d *Flow) NodeType() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.nodeRole
}

func (d *Flow) Network() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.network
}

// updateFromLogs consumes a container's logs for at most 5 seconds
// until it discovers both node role and network properties.
func (d *Flow) updateFromLogs(containerName string) error {
	if d.nodeRole != "" && d.network != "" {
		// TODO: node type discovery is quite expensive
		// and we are not handling role changes for now
		// so ignore further calls.
		return nil
	}

	reader, err := utils.DockerLogs(
		context.Background(),
		containerName,
		types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       tailLinesStr,
		},
	)
	if err != nil {
		return err
	}
	defer reader.Close()

	// We can't assume a single log line will have all the keys we need. Watch the log
	// at most for 5 seconds until all metadata has been extracted,
	started := time.Now()
	for time.Since(started) < 5*time.Second {
		// cleanup header from log line
		hdr := make([]byte, 8)
		_, err := reader.Read(hdr)
		if err != nil {
			return err
		}

		got, err := utils.GetLogLine(reader)
		if err != nil {
			return err
		}
		if len(got) == 0 {
			return fmt.Errorf("empty log line")
		}

		m := map[string]interface{}{}
		if err := json.Unmarshal(got, &m); err != nil {
			return err
		}

		if role, ok := m["node_role"]; ok && d.nodeRole == "" {
			d.nodeRole, ok = role.(string)
			if !ok {
				return fmt.Errorf("type assertion failed for node role: %v", role)
			}
		}

		if chain, ok := m["chain"]; ok && d.network == "" {
			d.network, ok = chain.(string)
			if !ok {
				return fmt.Errorf("type assertion failed for chain: %v", chain)
			}
		}

		if d.network != "" && d.nodeRole != "" {
			zap.S().Debugw("found both node_role and network", "node_role", d.nodeRole, "network", d.network)
			break
		}
	}

	zap.S().Debugw("node metadata discovery took ", "took", time.Since(started), "network", d.network, "node_role", d.nodeRole)

	if d.network == "" || d.nodeRole == "" {
		return fmt.Errorf("could not discover node role or network")
	}

	return nil
}

func (d *Flow) NodeVersion() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.nodeVersion
}

func (d *Flow) updateNodeVersion() (string, error) {
	if d.container == nil {
		return "", errors.New("node version: container not configured")
	}

	imageParts := strings.Split(d.container.Image, ":")
	if len(imageParts) < 2 {
		return "", fmt.Errorf("could not find node version: %v", d.container.Image)
	}

	d.nodeVersion = imageParts[1]

	return d.nodeVersion, nil
}

func (d *Flow) Protocol() string {
	return protocolName
}
