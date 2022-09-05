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

package global

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
	yaml "gopkg.in/yaml.v3"
)

var (
	// AgentConf the agent loaded configuration
	AgentConf AgentConfig

	// AppName name to use for directories
	AppName = "metrikad"

	// AppOptPath for agent binary
	AppOptPath = filepath.Join("/opt", AppName)

	// AppEtcPath for agent configuration files
	AppEtcPath = filepath.Join("/etc", AppName)

	// DefaultAgentConfigName config filename
	DefaultAgentConfigName = "agent.yml"

	// DefaultAgentConfigPath file path to load agent config from
	DefaultAgentConfigPath = filepath.Join(AppEtcPath, "configs", DefaultAgentConfigName)

	// DefaultFingerprintFilename filename to use for the agent's hostname
	DefaultFingerprintFilename = "ma_fingerprint"

	// AgentCacheDir directory for writing agent runtime data (i.e. hostname)
	AgentCacheDir string

	// AgentHostname the hostname detected
	AgentHostname string

	// DefaultRuntimeSamplingInterval default sampling interval
	DefaultRuntimeSamplingInterval = 5 * time.Second
)

// PlatformConfig platform specific configuration
type PlatformConfig struct {
	APIKey             string        `yaml:"api_key"`
	BatchN             int           `yaml:"batch_n"`
	TransportTimeout   time.Duration `yaml:"transport_timeout"`
	MaxPublishInterval time.Duration `yaml:"max_publish_interval"`
	Addr               string        `yaml:"addr"`
	URI                string        `yaml:"uri"`
	RetryCount         int           `yaml:"retry_count"`
}

// BufferConfig used for configuring data buffering by the agent.
type BufferConfig struct {
	MaxHeapAlloc uint64        `yaml:"max_heap_alloc"`
	TTL          time.Duration `yaml:"ttl"`
}

// WatchConfig watch configuration to override default sampling interval.
type WatchConfig struct {
	Type             string        `yaml:"type"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
}

// RuntimeConfig configuration related to the agent runtime.
type RuntimeConfig struct {
	MetricsAddr                  string         `yaml:"metrics_addr"`
	Log                          LogConfig      `yaml:"logging"`
	SamplingInterval             time.Duration  `yaml:"sampling_interval"`
	UseExporters                 bool           `yaml:"use_exporters"`
	Watchers                     []*WatchConfig `yaml:"watchers"`
	DisableFingerprintValidation bool           `yaml:"disable_fingerprint_validation"`
}

// AgentConfig wraps all config used by the agent
type AgentConfig struct {
	Platform PlatformConfig `yaml:"platform"`
	Buffer   BufferConfig   `yaml:"buffer"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
}

// LogConfig agent logging configuration.
type LogConfig struct {
	Lvl     string   `yaml:"level"`
	Outputs []string `yaml:"outputs"`
}

var zapLevelMapper = map[string]zapcore.Level{
	"debug":   zapcore.DebugLevel,
	"info":    zapcore.InfoLevel,
	"warning": zapcore.WarnLevel,
	"error":   zapcore.ErrorLevel,
	"dpanic":  zapcore.DPanicLevel,
	"panic":   zapcore.PanicLevel,
	"fatal":   zapcore.FatalLevel,
}

// Level returns the current zap logging level
func (l LogConfig) Level() zapcore.Level {
	return zapLevelMapper[l.Lvl]
}

// ConfigFilePriority list of paths to check for agent configuration
var ConfigFilePriority = []string{
	DefaultAgentConfigName,
	DefaultAgentConfigPath,
}

// LoadAgentConfig loads agent configuration.
func LoadAgentConfig() error {
	var (
		content []byte
		err     error
	)

	for _, fn := range ConfigFilePriority {
		content, err = ioutil.ReadFile(fn)
		if err == nil {
			break
		}
	}

	if err != nil {
		log.Printf("configuration file %s not found", DefaultAgentConfigName)

		return err
	}

	if err := yaml.Unmarshal(content, &AgentConf); err != nil {
		return err
	}

	if AgentConf.Runtime.SamplingInterval == 0 {
		AgentConf.Runtime.SamplingInterval = DefaultRuntimeSamplingInterval
	}

	for _, watchConf := range AgentConf.Runtime.Watchers {
		if watchConf.SamplingInterval == 0*time.Second {
			watchConf.SamplingInterval = AgentConf.Runtime.SamplingInterval
		}
	}

	if len(AgentConf.Platform.APIKey) == 0 {
		return fmt.Errorf("API key is missing from loaded config")
	}

	if err := createLogFolders(); err != nil {
		return err
	}

	return nil
}

func createLogFolders() error {
	for _, logPath := range AgentConf.Runtime.Log.Outputs {
		if strings.HasSuffix(logPath, "/") {
			return fmt.Errorf("invalid log output path ending with '/': %s", logPath)
		}
		pathSplit := strings.Split(logPath, "/")
		if len(pathSplit) == 1 {
			continue
		}
		folder := strings.Join(pathSplit[:len(pathSplit)-1], "/")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return err
		}
	}

	return nil
}

// GenerateConfigFromTemplate generate a new configuration from a given
// template to use for node discovery.
func GenerateConfigFromTemplate(templatePath, configPath string, config interface{}) error {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return err
	}

	configFile, err := os.Create(configPath)
	if err != nil {
		return err
	}

	return t.Execute(configFile, config)
}
