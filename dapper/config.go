package dapper

import (
	"agent/internal/pkg/global"
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v3"
)

const (
	DefaultDapperPath   = "./internal/pkg/global/dapper.yml"
	DefaultTemplatePath = "./configs/dapper.template"
)

var DapperConf *DapperConfig

type DapperConfig struct {
	configPath      string
	Client          string               `yaml:"client"`
	ContainerRegex  []string             `yaml:"containerRegex"`
	NodeID          string               `yaml:"nodeID"`
	MetricEndpoints []global.PEFEndpoint `yaml:"pefEndpoints"`
	EnvFilePath     string               `yaml:"envFile"`
}

func NewDapperConfig(configPath ...string) DapperConfig {
	var path string
	if len(configPath) == 0 {
		path = DefaultDapperPath
	} else {
		path = configPath[0]
	}
	return DapperConfig{
		configPath: path,
	}
}

func (d *DapperConfig) Load() (DapperConfig, error) {
	var conf DapperConfig
	content, err := ioutil.ReadFile(d.configPath)
	if err != nil {
		return DapperConfig{}, err
	}

	if err := yaml.Unmarshal(content, &conf); err != nil {
		return DapperConfig{}, err
	}

	DapperConf = &conf
	return conf, nil
}

// Default overrides the configuration file specified in configPath
// with the template preset, and then loads it in memory.
func (d *DapperConfig) Default() (DapperConfig, error) {
	if err := global.GenerateConfigFromTemplate("./configs/dapper.template", d.configPath, d); err != nil {
		return DapperConfig{}, fmt.Errorf("failed to generate default template: %w", err)
	}

	return d.Load()
}
