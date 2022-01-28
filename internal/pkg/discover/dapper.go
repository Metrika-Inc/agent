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

type DapperValidator struct {
	// items        map[string]ValidatorState
	config       global.DapperConfig
	renderNeeded bool // if any config value was empty but got updated
	container    *dt.Container
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
		// items: map[string]ValidatorState{
		// 	"client":       0,
		// 	"nodeID":       0,
		// 	"logFile":      0,
		// 	"pefEndpoints": 0,
		// },
		config: config,
	}
}

// func (d DapperValidator) ValidateAll() bool {
// 	for _, v := range d.items {
// 		if v != Valid {
// 			return false
// 		}
// 	}
// 	return true
// }
func (d *DapperValidator) Execute() error {
	if d.config.Client != "" && d.config.NodeID != "" && len(d.config.PEFEndpoints) != 0 {
		// Maybe do validation in every case?
		fmt.Println("Protocol is already configured, nothing to do here.")
		return nil
	}

	c, err := GetContainer(d.config.ContainerRegex)
	if err != nil {
		fmt.Println("Unable to find flow-go docker container, is it running? Will attempt to auto-configure anyway.")
	} else {
		d.container = &c
	}
	// fmt.Println("command:", c.Command)
	// fmt.Println("labels: ", c.Labels)

	// fmt.Printf("%+v\n", c)

	if err := d.DiscoverNodeID(); err != nil {
		fmt.Println("Node ID: not found")
	}
	if err := d.DiscoverPEFEndoints(c.Ports); err != nil {
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
	// TODO: falling back to environment file (/etc/flow/runtime-conf.env)
	return errors.New("not found")
}

func (d *DapperValidator) DiscoverPEFEndoints(ports []dt.Port) error {
	var found bool
	portsToTry := map[int]struct{}{
		8080: {},
	}
	for _, port := range ports {
		if port.PublicPort != 3569 && port.PublicPort != 9000 {
			portsToTry[int(port.PublicPort)] = struct{}{}
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
		return errors.New("not found")
	}
	return nil
}

// func (d DapperValidator) Validated(item string) bool {
// 	return d.items[item] == Valid
// }

// func (d DapperValidator) ValidateLogFile() ValidatorState {
// 	if d.config.LogFile == "" {
// 		d.items["logFile"] = Empty
// 		return Empty
// 	}
// 	if _, err := os.Stat(d.config.LogFile); err != nil {
// 		return Invalid
// 	}
// 	d.items["logFile"] = Valid
// 	return Valid
// }

func (d *DapperValidator) ValidateClient() ValidatorState {
	if d.config.Client == "" {
		// d.items["client"] = Empty
		return Empty
	}

	// flow-go is the only dapper client
	if d.config.Client != "flow-go" {
		// d.items["client"] = Invalid
		return Invalid
	}

	// d.items["client"] = Valid
	return Valid
}

func (d *DapperValidator) DiscoverClient() error {
	return nil
}
