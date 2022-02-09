package dapper

import (
	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	dt "github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const (
	FlowVersionKey        = "FLOW_GO_NODE_VERSION"
	FlowNodeIDKey         = "FLOW_GO_NODE_ID"
	FlowExecutionNodeKey  = "FLOW_NETWORK_EXECUTION_NODE"
	FlowCollectionNodeKey = "FLOW_NETWORK_COLLECTION_NODE"
)

// Dapper is responsible for discovery and validation
// of the agent's dapper-related configuration.
type Dapper struct {
	config       DapperConfig
	renderNeeded bool // if any config value was empty but got updated
	container    *dt.Container
	env          map[string]string
}

func NewDapper() (*Dapper, error) {
	dapper := &Dapper{}
	config := NewDapperConfig(DefaultDapperPath)
	var err error
	dapper.config, err = config.Load()
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		// configuration does not exist, create it
		dapper.config, err = config.Default()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return dapper, nil
}

func (d *Dapper) ResetConfig() error {
	var err error
	config := NewDapperConfig(DefaultDapperPath)
	d.config, err = config.Default()
	return err
}

func (d *Dapper) IsConfigured() bool {
	if d.config.Client != "" && d.config.NodeID != "" && d.isPEFConfigured() {
		zap.S().Debug("protocol is already configured, nothing to do here")
		return true
	}
	return false
}

func (d *Dapper) isPEFConfigured() bool {
	if len(d.config.PEFEndpoints) == 0 {
		zap.S().Fatal("pefEndpoints field should always have an entry; running agent with reset flag should populate it")
	}

	return len(d.config.PEFEndpoints[0].URL) != 0
}

func (d *Dapper) Discover() error {
	log := zap.S()
	log.Info("dapper not fully configured, starting discovery")

	env, err := utils.GetEnvFromFile(d.config.EnvFilePath)
	if err != nil {
		log.Warnw("failed to load environment file", zap.Error(err))
	} else {
		// we need an else block because env gets initialized and returned
		// even if an error is encountered
		d.env = env
	}

	containers, err := utils.GetRunningContainers()
	if err != nil {
		log.Warnw("cannot access docker daemon, will attempt to auto-configure anyway", zap.Error(err))
	} else {
		container, err := utils.MatchContainer(containers, d.config.ContainerRegex)
		if err != nil {
			log.Warnw("unable to find running flow-go docker container, will attempt to auto-configure anyway")
		} else {
			d.container = &container
		}
	}

	errs := &utils.AutoConfigError{}
	if err := d.Client(); err != nil {
		log.Error("could not find client name")
		errs.Append(err)
	}
	if err := d.NodeID(); err != nil {
		log.Error("could not find node ID")
		errs.Append(err)
	}
	if err := d.DiscoverPEFEndpoints(); err != nil {
		log.Error("could not find PEF metric endpoints")
		errs.Append(err)
	}

	if d.renderNeeded {
		if err := global.GenerateConfigFromTemplate("./configs/dapper.template",
			DefaultDapperPath, d.config); err != nil {
			log.Errorw("failed to generate the template", zap.Error(err))
			errs.Append(err)
		}
		DapperConf = &d.config
	}

	return errs.ErrIfAny()
}

func (d *Dapper) NodeID() error {
	log := zap.S()
	if d.config.NodeID != "" {
		log.Debugw("NodeID exists, skipping discovery", "node_id", d.config.NodeID)
		return nil
	}
	if d.container != nil {
		args := strings.Split(d.container.Command, " ")
		for i := 0; i < len(args); i++ {
			if strings.ToLower(args[i]) == "--nodeid" {
				if i+1 >= len(args) {
					log.Warnw("potentially invalid docker run command", "cmd", d.container.Command)
				}
				log.Infow("node id found", "node_id", args[i+1])
				d.config.NodeID = args[i+1]
				d.renderNeeded = true
				return nil
			}
		}
	}

	// fall back to environment file
	if d.env != nil {
		if nodeID, ok := d.env[FlowNodeIDKey]; ok {
			log.Infow("node id found", "node_id", nodeID)
			d.config.NodeID = nodeID
			d.renderNeeded = true
			return nil
		}
	}
	return errors.New("node ID not found")
}

func (d *Dapper) DiscoverPEFEndpoints() error {
	if d.isPEFConfigured() {
		zap.S().Debug("PEFEndpoints already exist, skipping discovery")
		return nil
	}
	var found bool
	portsToTry := map[int]struct{}{
		8080: {},
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

func (d *Dapper) ValidateClient() error {
	// flow-go is the only dapper client
	if d.config.Client != "flow-go" {
		return errors.New("invalid client specified")
	}
	return nil
}

func (d *Dapper) Client() error {
	if d.config.Client == "" {
		d.config.Client = "flow-go"
	}
	return nil
}

// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
func (d *Dapper) PEFEndpoints() []global.PEFEndpoint {
	return d.config.PEFEndpoints
}

// ContainerRegex returns a regex-compatible string to identify the blockchain node
// if it is running on a docker container
func (d *Dapper) ContainerRegex() []string {
	return d.config.ContainerRegex
}

// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
// TODO: change to models.FromContext when merging
func (d *Dapper) LogEventsList() map[string]model.FromContext {
	return eventsFromContext
}

// NodeLogPath returns the path to the log file to watch.
// TODO: string -> []string perhaps
func (d *Dapper) NodeLogPath() string {
	return "" // TODO: copy over from Events branch
}
