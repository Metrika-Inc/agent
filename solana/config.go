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

package solana

import (
	"fmt"

	"agent/internal/pkg/global"
)

const (
	// DefaultFlowPath default flow configuration path
	DefaultFlowPath = "/etc/metrikad/configs/solana.yml"

	// DefaultTemplatePath default flow template configuration path
	DefaultTemplatePath = "/etc/metrikad/configs/solana.template"

	// DefaultBlockchainPath for blockchain specific configuration files
	DefaultBlockchainPath = "solana"
)

var solanaConf *solanaConfig

type solanaConfig struct{}

func newSolanaConfig(configPath ...string) *solanaConfig {
	return &solanaConfig{}
}

func (d *solanaConfig) load() (*solanaConfig, error) {
	conf := &solanaConfig{}
	return conf, nil
}

// Default overrides the configuration file specified in configPath
// with the template preset, and then loads it in memory.
func (d *solanaConfig) Default() (*solanaConfig, error) {
	if err := global.GenerateConfigFromTemplate(DefaultTemplatePath, DefaultBlockchainPath, d); err != nil {
		return nil, fmt.Errorf("failed to generate default template: %w", err)
	}

	return d.load()
}
