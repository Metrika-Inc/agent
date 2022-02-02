package discover

import (
	"agent/internal/pkg/global"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	dt "github.com/docker/docker/api/types"
)

const (
	FlowVersionKey        = "FLOW_GO_NODE_VERSION"
	FlowNodeIDKey         = "FLOW_GO_NODE_ID"
	FlowExecutionNodeKey  = "FLOW_NETWORK_EXECUTION_NODE"
	FlowCollectionNodeKey = "FLOW_NETWORK_COLLECTION_NODE"
)

// DapperValidator is responsible for discovery and validation
// of the agent's dapper-related configuration.
type DapperValidator struct {
	// items        map[string]ValidatorState
	config       global.DapperConfig
	renderNeeded bool // if any config value was empty but got updated
	container    *dt.Container
	env          map[string]string
}

// dapperDiscovery helps to find and validate a running
// node client configuration.
func dapperDiscovery() {
	if err := global.LoadDapperConfig(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// config file doesn't exist, create one
			global.DapperConf = (&global.DapperConfig{}).Default()
		} else {
			panic(err)
		}
	}

	validator := NewDapperValidator(*global.DapperConf)
	if err := validator.Execute(); err != nil {
		fmt.Println("encountered errors verifying the configuration:", err)
	}

	if validator.renderNeeded {
		generateConfig("./configs/dapper.template", global.DefaultDapperPath, validator.config)
		global.DapperConf = &validator.config
	}
}

func NewDapperValidator(config global.DapperConfig) *DapperValidator {
	return &DapperValidator{
		config: config,
	}
}

func (d *DapperValidator) Execute() error {
	if d.config.Client != "" && d.config.NodeID != "" && len(d.config.PEFEndpoints) != 0 {
		// Maybe do validation in every case?
		// Validating everytime could be toggleable, but what's the default?
		// Answering "what does agent do if it's started when node is not running?" would help.
		fmt.Println("Protocol is already configured, nothing to do here.")
		return nil
	}

	env, err := getEnvFromFile(d.config.EnvFilePath)
	if err == nil {
		d.env = env
	}

	containers, err := GetRunningContainers()
	if err != nil {
		fmt.Println("Cannot access docker daemon, error:", err)
		fmt.Println("Will attempt to auto-configure anyway")
	} else {
		container, err := MatchContainer(containers, d.config.ContainerRegex)
		if err != nil {
			fmt.Println("Unable to find flow-go docker container, is it running? Will attempt to auto-configure anyway.")
		} else {
			d.container = &container
		}
	}

	if err := d.DiscoverNodeID(); err != nil {
		fmt.Println("Node ID: not found")
	}
	if err := d.DiscoverPEFEndoints(); err != nil {
		fmt.Println("PEF Metrics: not found")
	}

	// TODO: multi-error and propagation: tell what components could not be automatically discovered.

	return nil
}

func (d *DapperValidator) DiscoverNodeID() error {
	if d.container != nil {
		args := strings.Split(d.container.Command, " ")
		for i := 0; i < len(args); i++ {
			if strings.ToLower(args[i]) == "--nodeid" {
				if i+1 >= len(args) {
					fmt.Println("potentially invalid docker run command")
				}
				fmt.Printf("Node ID: Found (%s)\n", args[i+1])
				d.config.NodeID = args[i+1]
				d.renderNeeded = true
				return nil
			}
		}
	}

	// fall back to environment file
	if d.env != nil {
		if nodeID, ok := d.env[FlowNodeIDKey]; ok {
			d.config.NodeID = nodeID
			return nil
		}
	}
	return errors.New("node ID not found")
}

func (d *DapperValidator) DiscoverPEFEndoints() error {
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

	for port := range portsToTry {
		hostname := "http://127.0.0.1:" + strconv.Itoa(port) + "/metrics"

		resp, err := cl.Get(hostname)
		if err != nil {
			continue
		}
		if resp.StatusCode <= 204 {
			fmt.Printf("PEF Metrics: Found (%s)\n", hostname)
			d.config.PEFEndpoints = append(d.config.PEFEndpoints, hostname)
			found = true
			d.renderNeeded = true
		}
	}

	if !found {
		return errors.New("no PEF endpoints found")
	}
	return nil
}

func (d *DapperValidator) ValidateClient() error {
	// flow-go is the only dapper client
	if d.config.Client != "flow-go" {
		return errors.New("invalid client specified")
	}
	return nil
}

func (d *DapperValidator) DiscoverClient() error {
	if d.config.Client == "" {
		d.config.Client = "flow-go"
	}
	return nil
}
