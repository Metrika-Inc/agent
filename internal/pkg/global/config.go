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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
	yaml "gopkg.in/yaml.v3"

	"github.com/pkg/errors"
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

	// PlatformAPIKeyConfigPlaceholder config placeholder for dynamic api key configuration
	PlatformAPIKeyConfigPlaceholder = "<api_key>"

	// PlatformAddrConfigPlaceholder config placeholder for dynamic platform address configuration
	PlatformAddrConfigPlaceholder = "<platform_addr>"

	// PlatformAPIKeyEnvVar env var used to override platform.api_key
	PlatformAPIKeyEnvVar = "MA_API_KEY"

	// PlatformAddrEnvVar env var used to override platform.addr
	PlatformAddrEnvVar = "MA_PLATFORM"

	// DefaultRuntimeSamplingInterval default sampling interval
	DefaultRuntimeSamplingInterval = 5 * time.Second

	// DefaultPlatformBatchN default number of metrics+events to batch per publish
	DefaultPlatformBatchN = 1000

	// DefaultPlatformMaxPublishInterval default max period to wait
	// between two consecutive publish operations.
	DefaultPlatformMaxPublishInterval = 5 * time.Second

	// DefaultPlatformTransportTimeout default publish timeout
	DefaultPlatformTransportTimeout = 10 * time.Second

	// DefaultBufferMaxHeapAlloc max heap allocated objects
	DefaultBufferMaxHeapAlloc = uint64(52428800)

	// DefaultBufferMinBufferSize default min number of items to allow in the buffer
	DefaultBufferMinBufferSize = 2500

	// DefaultBufferTTL default buffer TTL, use zero for no TTL.
	DefaultBufferTTL = time.Duration(0)

	// DefaultPlatformURI (for http platform endpoints only)
	DefaultPlatformURI = "/"

	// DefaultRuntimeLoggingLevel default logging level
	DefaultRuntimeLoggingLevel = "warning"

	// DefaultRuntimeDisableFingerprintValidation default fingerprint validation policy
	DefaultRuntimeDisableFingerprintValidation = false

	// DefaultRuntimeMetricsAddr default address to expose Prometheus metrics
	DefaultRuntimeMetricsAddr = "127.0.0.1:9000"

	// DefaultRuntimeUseExporters default exporter usage
	DefaultRuntimeUseExporters = false

	// ConfigEnvPrefix prefix used for agent specific env vars
	ConfigEnvPrefix = "MA"
)

var (
	// DefaultRuntimeLoggingOutputs default log outputs
	DefaultRuntimeLoggingOutputs = []string{"stdout"}

	// DefaultRuntimeWatchers watchers enabled by default
	DefaultRuntimeWatchers = []*WatchConfig{
		{Type: "prometheus.proc.cpu"},
		{Type: "prometheus.proc.net.netstat_linux"},
		{Type: "prometheus.proc.net.arp_linux"},
		{Type: "prometheus.proc.stat_linux"},
		{Type: "prometheus.proc.conntrack_linux"},
		{Type: "prometheus.proc.diskstats"},
		{Type: "prometheus.proc.entropy"},
		{Type: "prometheus.proc.filefd"},
		{Type: "prometheus.proc.filesystem"},
		{Type: "prometheus.proc.loadavg"},
		{Type: "prometheus.proc.meminfo"},
		{Type: "prometheus.proc.netclass"},
		{Type: "prometheus.proc.netdev"},
		{Type: "prometheus.proc.sockstat"},
		{Type: "prometheus.proc.textfile"},
		{Type: "prometheus.time"},
		{Type: "prometheus.uname"},
		{Type: "prometheus.vmstat"},
	}
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
	IsEnabled          *bool         `yaml:"enabled"`
}

// BufferConfig used for configuring data buffering by the agent.
type BufferConfig struct {
	MaxHeapAlloc  uint64        `yaml:"max_heap_alloc"`
	MinBufferSize int           `yaml:"min_buffer_size"`
	TTL           time.Duration `yaml:"ttl"`
}

// WatchConfig watch configuration to override default sampling interval.
type WatchConfig struct {
	Type             string        `yaml:"type"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
}

// RuntimeConfig configuration related to the agent runtime.
type RuntimeConfig struct {
	MetricsAddr                  string                 `yaml:"metrics_addr"`
	Log                          LogConfig              `yaml:"logging"`
	SamplingInterval             time.Duration          `yaml:"sampling_interval"`
	UseExporters                 bool                   `yaml:"use_exporters"`
	Watchers                     []*WatchConfig         `yaml:"watchers"`
	DisableFingerprintValidation bool                   `yaml:"disable_fingerprint_validation"`
	ExportersRaw                 map[string]interface{} `yaml:"exporters"`
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

// overloadFromEnv tries to parse all configuration parameters from the environment
// and always overrides global config if a value is set.
func overloadFromEnv() error {
	v := os.Getenv(PlatformAPIKeyEnvVar)
	if v != "" {
		AgentConf.Platform.APIKey = v
	}

	v = os.Getenv(PlatformAddrEnvVar)
	if v != "" {
		AgentConf.Platform.Addr = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_batch_n"))
	if v != "" {
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "platform_batch_n env parse error")
		}
		AgentConf.Platform.BatchN = int(vInt)
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_max_publish_interval"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "platform_max_publish_internval env parse error")
		}
		AgentConf.Platform.MaxPublishInterval = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_transport_timeout"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "platform_transport_timeout env parse error")
		}
		AgentConf.Platform.TransportTimeout = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_uri"))
	if v != "" {
		AgentConf.Platform.URI = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_max_heap_alloc"))
	if v != "" {
		vUint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "buffer_max_heap_alloc env parse error")
		}
		AgentConf.Buffer.MaxHeapAlloc = vUint
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_min_buffer_size"))
	if v != "" {
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "buffer_min_buffer_size env parse error")
		}
		AgentConf.Buffer.MinBufferSize = int(vInt)
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_ttl"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "buffer_ttl env parse error")
		}
		AgentConf.Buffer.TTL = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_logging_outputs"))
	if v != "" {
		AgentConf.Runtime.Log.Outputs = strings.Split(v, ",")
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_logging_level"))
	if v != "" {
		AgentConf.Runtime.Log.Lvl = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_disable_fingerprint_validation"))
	if v != "" {
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_disable_fingeprint_validation env parse error")
		}

		AgentConf.Runtime.DisableFingerprintValidation = vBool
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_metrics_addr"))
	if v != "" {
		AgentConf.Runtime.MetricsAddr = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_sampling_interval"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_sampling_interval env parse error")
		}
		AgentConf.Runtime.SamplingInterval = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_use_exporters"))
	if v != "" {
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_use_exporters env parse error")
		}

		AgentConf.Runtime.UseExporters = vBool
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_watchers"))
	if v != "" {
		watchers := []*WatchConfig{}
		vwatchers := strings.Split(v, ",")
		for _, vw := range vwatchers {
			watchers = append(watchers, &WatchConfig{Type: vw})
		}

		AgentConf.Runtime.Watchers = watchers
	}

	return nil
}

// ensureDefaults ensures sane defaults in global agent configuration
func ensureDefaults() {
	if AgentConf.Runtime.SamplingInterval == 0 {
		AgentConf.Runtime.SamplingInterval = DefaultRuntimeSamplingInterval
	}

	for _, watchConf := range AgentConf.Runtime.Watchers {
		if watchConf.SamplingInterval == 0*time.Second {
			watchConf.SamplingInterval = AgentConf.Runtime.SamplingInterval
		}
	}

	if AgentConf.Platform.BatchN == 0 {
		AgentConf.Platform.BatchN = DefaultPlatformBatchN
	}

	if AgentConf.Platform.MaxPublishInterval == 0 {
		AgentConf.Platform.MaxPublishInterval = DefaultPlatformMaxPublishInterval
	}

	if AgentConf.Platform.TransportTimeout == 0 {
		AgentConf.Platform.TransportTimeout = DefaultPlatformTransportTimeout
	}

	if AgentConf.Platform.URI == "" {
		AgentConf.Platform.URI = DefaultPlatformURI
	}

	if AgentConf.Buffer.MaxHeapAlloc == 0 {
		AgentConf.Buffer.MaxHeapAlloc = DefaultBufferMaxHeapAlloc
	}

	if AgentConf.Buffer.MinBufferSize == 0 {
		AgentConf.Buffer.MinBufferSize = DefaultBufferMinBufferSize
	}

	if AgentConf.Buffer.TTL == 0 {
		AgentConf.Buffer.TTL = DefaultBufferTTL
	}

	if len(AgentConf.Runtime.Log.Outputs) == 0 {
		AgentConf.Runtime.Log.Outputs = DefaultRuntimeLoggingOutputs
	}

	if AgentConf.Runtime.Log.Lvl == "" {
		AgentConf.Runtime.Log.Lvl = DefaultRuntimeLoggingLevel
	}

	if AgentConf.Runtime.MetricsAddr == "" {
		AgentConf.Runtime.MetricsAddr = DefaultRuntimeMetricsAddr
	}

	if AgentConf.Runtime.SamplingInterval == 0 {
		AgentConf.Runtime.SamplingInterval = DefaultRuntimeSamplingInterval
	}

	if len(AgentConf.Runtime.Watchers) == 0 {
		AgentConf.Runtime.Watchers = DefaultRuntimeWatchers
	}
}

// ensureRequired ensures global agent configuration has loaded required configuration
func ensureRequired() error {
	if AgentConf.Platform.Addr == "" || AgentConf.Platform.Addr == PlatformAddrConfigPlaceholder {
		return fmt.Errorf("platform.addr is missing from loaded config")
	}

	if AgentConf.Platform.APIKey == "" || AgentConf.Platform.APIKey == PlatformAPIKeyConfigPlaceholder {
		return fmt.Errorf("platform.api_key is missing from loaded config")
	}

	return nil
}

// LoadAgentConfig loads agent configuration in the following priority:
// 1. Load configuration from the first file found in ConfigFilePriority.
// 2. Override any configuration key if an environment variable is set.
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

	if err := yaml.Unmarshal(content, &AgentConf); err != nil {
		return err
	}

	if err := overloadFromEnv(); err != nil {
		return errors.Wrapf(err, "error while loading config from env")
	}

	ensureDefaults()

	if err := ensureRequired(); err != nil {
		return errors.Wrapf(err, "loaded configuration is missing a required parameter")
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

// Enabled checks if exporting to the metrika platform is Enabled
// in agent's configuration.
// Default: true.
func (p *PlatformConfig) Enabled() bool {
	if p.IsEnabled == nil {
		return true
	}
	return *p.IsEnabled
}
