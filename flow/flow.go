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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const (
	flowVersionKey        = "FLOW_GO_NODE_VERSION"
	flowNodeIDKey         = "FLOW_GO_NODE_ID"
	flowExecutionNodeKey  = "FLOW_NETWORK_EXECUTION_NODE"
	flowCollectionNodeKey = "FLOW_NETWORK_COLLECTION_NODE"

	// protocolName blockchain protocol name
	protocolName = "flow"

	// tailLinesStr is the default number of lines to fetch
	// when peeking the logfile for network/chain or node role data.
	tailLinesStr = "100"
	tailLinesInt = 100

	// Valid node roles
	nodeRoleAccess       = "access"
	nodeRoleCollection   = "collection"
	nodeRoleConsensus    = "consensus"
	nodeRoleExecution    = "execution"
	nodeRoleVerification = "verification"
)

var recognizedNodeRoles = map[string]struct{}{
	"":                   {}, // valid value when node not discovered yet
	nodeRoleAccess:       {},
	nodeRoleCollection:   {},
	nodeRoleConsensus:    {},
	nodeRoleExecution:    {},
	nodeRoleVerification: {},
}

// Flow is responsible for discovery and validation
// of the agent's flow-related configuration.
// Implements global.Chain.
type Flow struct {
	config          flowConfig
	renderNeeded    bool // if any config value was empty but got updated
	container       *types.Container
	systemdService  *dbus.UnitStatus
	env             map[string]string
	nodeRole        string
	network         string
	nodeVersion     string
	mutex           *sync.RWMutex
	runScheme       global.NodeRunScheme
	configUpdatesCh chan global.ConfigUpdate
}

// NewFlow flow chain constructor.
func NewFlow() (*Flow, error) {
	flow := &Flow{mutex: &sync.RWMutex{}}
	config := newFlowConfig(DefaultFlowPath)
	var err error
	flow.config, err = config.load()
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		// configuration does not exist, create it
		flow.config, err = config.Default()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	flow.configUpdatesCh = make(chan global.ConfigUpdate, 1)

	return flow, nil
}

// ResetConfig resets discovered configuration to default.
func (d *Flow) ResetConfig() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var err error
	config := newFlowConfig(DefaultFlowPath)
	d.config, err = config.Default()
	return err
}

// IsConfigured depends on Client, NodeID and PEF configuration to be present.
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

// NodeID returns the discovered node id.
func (d *Flow) NodeID() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.NodeID
}

func (d *Flow) configureNodeIDFromEnvFile() (string, error) {
	env, err := utils.GetEnvFromFile(d.config.EnvFilePath)
	if err != nil {
		zap.S().Errorw("failed to load environment file", zap.Error(err))
	} else {
		// we need an else block because env gets initialized and returned
		// even if an error is encountered
		d.env = env
	}

	if d.env == nil {
		return "", fmt.Errorf("got nil env when loading from env file: %s", d.config.EnvFilePath)
	}

	nodeID, ok := d.env[flowNodeIDKey]
	if !ok {
		return "", errors.New("node ID not found")
	}

	zap.S().Infow("node id found", "node_id", nodeID)
	d.config.NodeID = nodeID
	d.renderNeeded = true

	return d.config.NodeID, nil
}

func (d *Flow) configureNodeIDFromDocker() (string, error) {
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
		defer resp.Body.Close()
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

// ValidateClient validates client name
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

// PEFEndpoints returns a list of endpoints for fetching PEF metrics.
func (d *Flow) PEFEndpoints() []global.PEFEndpoint {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.PEFEndpoints
}

// ContainerRegex Deprecated: use discovery.hints.docker instead.
func (d *Flow) ContainerRegex() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.config.ContainerRegex
}

// LogEventsList map of events being tracked from the node log
func (d *Flow) LogEventsList() map[string]model.FromContext {
	return eventsFromContext
}

// NodeLogPath Note: to be implemented with linux process discovery.
func (d *Flow) NodeLogPath() string {
	return ""
}

// NodeRole returns the discovered node role (i.e. consensus).
func (d *Flow) NodeRole() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.nodeRole
}

// Network returns the discovered network (i.e. flow-localnet).
func (d *Flow) Network() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.network
}

func (d *Flow) updateMetadataFromJSON(b []byte) error {
	m := map[string]interface{}{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	if role, ok := m["node_role"]; ok && d.nodeRole == "" {
		nodeRole, ok := role.(string)
		if !ok {
			return fmt.Errorf("type assertion failed for node role: %v", role)
		}
		nodeRole = strings.ToLower(nodeRole)

		if _, ok := recognizedNodeRoles[nodeRole]; !ok {
			return fmt.Errorf("unsupported node role detected: %s", nodeRole)
		}

		d.nodeRole = nodeRole
	}

	d.updateNetworkFromJSONLog(m)

	return nil
}

func (d *Flow) updateFromLogs(reader io.ReadCloser, hdrLen int) error {
	// We can't assume a single log line will have all the keys we need. Watch the log
	// at most for 5 seconds until all metadata has been extracted,
	started := time.Now()
	scan := bufio.NewScanner(reader)
	for time.Since(started) < 5*time.Second {
		got, err := utils.GetLogLine(scan)
		if err != nil {
			return err
		}

		// This assumes the container is not using a TTY. In this case
		// stdout/stderr are multiplexed on the same stream and 8-byte header
		// precedes each line. We don't need to parse the header since we are
		// using scanner.Bytes().
		if got == nil || (hdrLen > 0 && len(got) < hdrLen) {
			return fmt.Errorf("empty log line")
		}

		got = got[hdrLen:]

		if string(got[0]) != "{" {
			continue
		}

		m := map[string]interface{}{}
		if err := json.Unmarshal(got, &m); err != nil {
			return err
		}

		if err := d.updateMetadataFromJSON(got); err != nil {
			return err
		}

		if d.network != "" && d.nodeRole != "" {
			break
		}
	}

	if d.network == "" || d.nodeRole == "" {
		zap.S().Warnw("missing metadata", "network", d.network, "node_role", d.nodeRole)

		return fmt.Errorf("could not discover node role or network")
	}

	zap.S().Infow("metadata discovered", "network", d.network, "node_role", d.nodeRole, "took", time.Since(started))

	return nil
}

// access nodes (and potentially others) use chain_id to log the network
var networkLogKeyCandidates = []string{"chain", "chain_id"}

// updateNetworkFromJSONLog update the network by looking for a known value
// in a list of keys. Network is updated based on the first key with a value
// containing with a known network.
func (d *Flow) updateNetworkFromJSONLog(m map[string]interface{}) {
	for _, key := range networkLogKeyCandidates {
		if d.network != "" {
			return
		}

		if chain, ok := m[key]; ok {
			networkVal, ok := chain.(string)
			if !ok {
				zap.S().Warnw("type assertion failed for chain log field", "chain", chain)

				continue
			}

			if utils.KnownNetwork(networkVal) {
				d.network = networkVal
				break
			}
		}
	}
}

// NodeVersion returns the discovered node version
func (d *Flow) NodeVersion() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.nodeVersion
}

func (d *Flow) updateNodeVersionFromDocker() (string, error) {
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

// Protocol returns "flow"
func (d *Flow) Protocol() string {
	return protocolName
}

// LogWatchEnabled specifies if a log watcher should be turned on or not.
// Currently we're only collecting flow node logs if flow node type is consensus.
//
// Note: if node type is not yet discovered, LogWatchEnabled will return false.
func (d *Flow) LogWatchEnabled() bool {
	return d.nodeRole == nodeRoleConsensus
}

func (d *Flow) pushConfigUpdate(upd global.ConfigUpdate) {
	// push config update
	select {
	case d.configUpdatesCh <- upd:
	default:
		zap.S().Warnw("config update chan blocked update, dropping", upd)
	}
}

func (d *Flow) reconfigureSystemd(reader io.ReadCloser) error {
	log := zap.S()
	errs := &utils.AutoConfigError{}

	if _, err := d.configureNodeIDFromEnvFile(); err != nil {
		log.Warnw("could not find node ID in env file", zap.Error(err))
		errs.Append(err)
	}

	if err := d.configurePEFEndpoints(); err != nil {
		log.Warn("could not find PEF metric endpoints")
		errs.Append(err)
	} else {
		upd := global.ConfigUpdate{Key: global.PEFEndpointsKey, Val: d.config.PEFEndpoints}
		d.pushConfigUpdate(upd)
	}

	if d.systemdService != nil && d.systemdService.SubState == "running" {
		if err := d.updateFromLogs(reader, 0); err != nil {
			log.Warnw("error getting node metadata from journal logs", zap.Error(err))
			errs.Append(err)
		}
	}

	// TODO: get node version from prometheus metric

	if d.renderNeeded {
		if err := global.GenerateConfigFromTemplate(DefaultTemplatePath, DefaultFlowPath, d.config); err != nil {
			log.Errorw("failed to generate the template", zap.Error(err))
			// errs.Append(err)
		}
		d.renderNeeded = false
		flowConf = &d.config
	}

	err := errs.ErrIfAny()
	if err != nil {
		return err
	}

	log.Infow("node configuration successful (systemd)", "network", d.network, "node_role", d.nodeRole)

	return nil
}

func (d *Flow) reconfigureDocker(reader io.ReadCloser) error {
	errs := &utils.AutoConfigError{}
	log := zap.S()

	if err := d.configureClient(); err != nil {
		log.Warn("could not find client name")

		errs.Append(err)
	}

	if _, err := d.configureNodeIDFromDocker(); err != nil {
		log.Warnw("could not find node ID in docker container cmd args", zap.Error(err))
		errs.Append(err)

		// fall back to environment file
		if _, err := d.configureNodeIDFromEnvFile(); err != nil {
			log.Warnw("could not find node ID in env file", zap.Error(err))
			errs.Append(err)
		}
	}

	if _, err := d.updateNodeVersionFromDocker(); err != nil {
		log.Warn("could not find node version")
		// errs.Append(err)
	}

	if err := d.configurePEFEndpoints(); err != nil {
		log.Warn("could not find PEF metric endpoints")
		errs.Append(err)
	} else {
		upd := global.ConfigUpdate{Key: global.PEFEndpointsKey, Val: d.config.PEFEndpoints}
		d.pushConfigUpdate(upd)
	}

	if d.container != nil && len(d.container.Names) > 0 {
		if err := d.updateFromLogs(reader, 8); err != nil {
			log.Warnw("error getting node metadata from docker logs", zap.Error(err))
			errs.Append(err)
		}
	}

	if d.renderNeeded {
		if err := global.GenerateConfigFromTemplate(DefaultTemplatePath,
			DefaultFlowPath, d.config); err != nil {
			log.Errorw("failed to generate the template", zap.Error(err))
			// errs.Append(err)
		}
		d.renderNeeded = false
		flowConf = &d.config
	}

	err := errs.ErrIfAny()
	if err != nil {
		return err
	}

	log.Infow("node configuration successful (docker)", "network", d.network, "node_role", d.nodeRole)

	return nil
}

// ReconfigureByDockerContainer re-runs the configuration process for all node
// metadata (i.e. node version) using container metadata and logs.
func (d *Flow) ReconfigureByDockerContainer(container *types.Container, reader io.ReadCloser) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.network != "" && d.nodeRole != "" {
		return nil
	}

	d.container = container

	if err := d.reconfigureDocker(reader); err != nil {
		return err
	}

	return nil
}

// ReconfigureBySystemdUnit re-runs the configuration process for all node
// metadata (i.e. node version) using unit metadata and journal logs.
func (d *Flow) ReconfigureBySystemdUnit(unit *dbus.UnitStatus, reader io.ReadCloser) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.network != "" && d.nodeRole != "" {
		return nil
	}

	d.systemdService = unit

	if err := d.reconfigureSystemd(reader); err != nil {
		return err
	}

	return nil
}

// SetRunScheme sets the node run scheme
func (d *Flow) SetRunScheme(s global.NodeRunScheme) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.runScheme = s
}

// SetDockerContainer sets the node's container object
func (d *Flow) SetDockerContainer(container *types.Container) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.container = container
}

// SetSystemdService sets the node's systemd service unit
func (d *Flow) SetSystemdService(unit *dbus.UnitStatus) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.systemdService = unit
}

// ConfigUpdateCh returns the channel used to emit configuration updates
func (d *Flow) ConfigUpdateCh() chan global.ConfigUpdate {
	return d.configUpdatesCh
}
