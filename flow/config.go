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
	"fmt"
	"io/ioutil"

	"agent/internal/pkg/global"

	yaml "gopkg.in/yaml.v3"
)

var (
	// DefaultFlowPath default flow configuration path
	DefaultFlowPath = "/etc/metrikad/configs/flow.yml"

	// DefaultTemplatePath default flow template configuration path
	DefaultTemplatePath = "/etc/metrikad/configs/flow.template"
)

var flowConf *flowConfig

type flowConfig struct {
	configPath     string
	Client         string               `yaml:"client"`
	ContainerRegex []string             `yaml:"containerRegex"`
	NodeID         string               `yaml:"nodeID"`
	PEFEndpoints   []global.PEFEndpoint `yaml:"pefEndpoints"`
	EnvFilePath    string               `yaml:"envFile"`
}

func newFlowConfig(configPath ...string) flowConfig {
	var path string
	if len(configPath) == 0 {
		path = DefaultFlowPath
	} else {
		path = configPath[0]
	}
	return flowConfig{
		configPath: path,
	}
}

func (d *flowConfig) load() (flowConfig, error) {
	var conf flowConfig
	content, err := ioutil.ReadFile(d.configPath)
	if err != nil {
		return flowConfig{}, err
	}

	if err := yaml.Unmarshal(content, &conf); err != nil {
		return flowConfig{}, err
	}

	flowConf = &conf
	return conf, nil
}

// Default overrides the configuration file specified in configPath
// with the template preset, and then loads it in memory.
func (d *flowConfig) Default() (flowConfig, error) {
	if err := global.GenerateConfigFromTemplate(DefaultTemplatePath, d.configPath, d); err != nil {
		return flowConfig{}, fmt.Errorf("failed to generate default template: %w", err)
	}

	return d.load()
}
