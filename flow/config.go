package flow

import (
	"fmt"
	"io/ioutil"

	"agent/internal/pkg/global"

	yaml "gopkg.in/yaml.v3"
)

const (
	DefaultFlowPath   = "/etc/metrikad/configs/flow.yml"
	DefaultTemplatePath = "/etc/metrikad/configs/flow.template"
)

var FlowConf *FlowConfig

type FlowConfig struct {
	configPath     string
	Client         string               `yaml:"client"`
	ContainerRegex []string             `yaml:"containerRegex"`
	NodeID         string               `yaml:"nodeID"`
	PEFEndpoints   []global.PEFEndpoint `yaml:"pefEndpoints"`
	EnvFilePath    string               `yaml:"envFile"`
}

func NewFlowConfig(configPath ...string) FlowConfig {
	var path string
	if len(configPath) == 0 {
		path = DefaultFlowPath
	} else {
		path = configPath[0]
	}
	return FlowConfig{
		configPath: path,
	}
}

func (d *FlowConfig) Load() (FlowConfig, error) {
	var conf FlowConfig
	content, err := ioutil.ReadFile(d.configPath)
	if err != nil {
		return FlowConfig{}, err
	}

	if err := yaml.Unmarshal(content, &conf); err != nil {
		return FlowConfig{}, err
	}

	FlowConf = &conf
	return conf, nil
}

// Default overrides the configuration file specified in configPath
// with the template preset, and then loads it in memory.
func (d *FlowConfig) Default() (FlowConfig, error) {
	if err := global.GenerateConfigFromTemplate(DefaultTemplatePath, d.configPath, d); err != nil {
		return FlowConfig{}, fmt.Errorf("failed to generate default template: %w", err)
	}

	return d.Load()
}
