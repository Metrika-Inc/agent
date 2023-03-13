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
	"os/user"
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

	// LocalAgentConfigName ./configs/agent.yml mostly used for testing
	LocalAgentConfigName = filepath.Join("configs", DefaultAgentConfigName)

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
	DefaultRuntimeSamplingInterval = 15 * time.Second

	// DefaultPlatformEnabled default platform exporter's enabled state
	DefaultPlatformEnabled = true

	// DefaultPlatformBatchN default number of metrics+events to batch per publish
	DefaultPlatformBatchN = 1000

	// DefaultPlatformMaxPublishInterval default max period to wait
	// between two consecutive publish operations.
	DefaultPlatformMaxPublishInterval = 15 * time.Second

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

	// DefaultRuntimeHTTPAddr default address to expose Prometheus metrics
	DefaultRuntimeHTTPAddr = ""

	// DefaultRuntimeHostHeaderValidationEnabled default HTTP host header validation
	DefaultRuntimeHostHeaderValidationEnabled = true

	// DefaultRuntimeAllowedHosts default list of allowed HTTP host headers
	DefaultRuntimeAllowedHosts = []string{"127.0.0.1"}

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
	Enabled            *bool         `yaml:"enabled"`
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
	HTTPAddr                     string                 `yaml:"http_addr"`
	MetricsEnabled               bool                   `yaml:"metrics_enabled"`
	HostHeaderValidationEnabled  *bool                  `yaml:"host_header_validation_enabled"`
	AllowedHosts                 []string               `yaml:"allowed_hosts"`
	Log                          LogConfig              `yaml:"logging"`
	SamplingInterval             time.Duration          `yaml:"sampling_interval"`
	Watchers                     []*WatchConfig         `yaml:"watchers"`
	DisableFingerprintValidation bool                   `yaml:"disable_fingerprint_validation"`
	Exporters                    map[string]interface{} `yaml:"exporters"`
}

// Hints node discovery hints
type Hints struct {
	Systemd []string `yaml:"systemd"`
	Docker  []string `yaml:"docker"`
}

// DiscoverySystemd systemd discovery configuration
type DiscoverySystemd struct {
	Deactivated bool     `yaml:"deactivated"`
	Glob        []string `yaml:"glob"`
}

// DiscoveryDocker docker discovery configuration
type DiscoveryDocker struct {
	Deactivated bool     `yaml:"deactivated"`
	Regex       []string `yaml:"regex"`
}

// DiscoveryConfig configuration related to node discovery.
type DiscoveryConfig struct {
	Docker  DiscoveryDocker  `yaml:"docker"`
	Systemd DiscoverySystemd `yaml:"systemd"`
}

// AgentConfig wraps all config used by the agent
type AgentConfig struct {
	Platform  PlatformConfig  `yaml:"platform"`
	Buffer    BufferConfig    `yaml:"buffer"`
	Runtime   RuntimeConfig   `yaml:"runtime"`
	Discovery DiscoveryConfig `yaml:"discovery"`
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
	LocalAgentConfigName,
	DefaultAgentConfigPath,
}

// overloadFromEnv tries to parse all configuration parameters from the environment
// and always overrides global config if a value is set.
func overloadFromEnv(c *AgentConfig) error {
	v := os.Getenv(PlatformAPIKeyEnvVar)
	if v != "" {
		c.Platform.APIKey = v
	}

	v = os.Getenv(PlatformAddrEnvVar)
	if v != "" {
		c.Platform.Addr = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_enabled"))
	if v != "" {
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Wrapf(err, "platform_enabled env parse error")
		}
		c.Platform.Enabled = &vBool
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_batch_n"))
	if v != "" {
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "platform_batch_n env parse error")
		}
		c.Platform.BatchN = int(vInt)
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_max_publish_interval"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "platform_max_publish_internval env parse error")
		}
		c.Platform.MaxPublishInterval = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_transport_timeout"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "platform_transport_timeout env parse error")
		}
		c.Platform.TransportTimeout = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "platform_uri"))
	if v != "" {
		c.Platform.URI = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_max_heap_alloc"))
	if v != "" {
		vUint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "buffer_max_heap_alloc env parse error")
		}
		c.Buffer.MaxHeapAlloc = vUint
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_min_buffer_size"))
	if v != "" {
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "buffer_min_buffer_size env parse error")
		}
		c.Buffer.MinBufferSize = int(vInt)
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "buffer_ttl"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "buffer_ttl env parse error")
		}
		c.Buffer.TTL = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_logging_outputs"))
	if v != "" {
		c.Runtime.Log.Outputs = strings.Split(v, ",")
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_logging_level"))
	if v != "" {
		c.Runtime.Log.Lvl = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_disable_fingerprint_validation"))
	if v != "" {
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_disable_fingeprint_validation env parse error")
		}

		c.Runtime.DisableFingerprintValidation = vBool
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_host_header_validation_enabled"))
	if v != "" {
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_host_header_validation_enabled env parse error")
		}

		c.Runtime.HostHeaderValidationEnabled = &vBool
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_http_addr"))
	if v != "" {
		c.Runtime.HTTPAddr = v
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_allowed_hosts"))
	if v != "" {
		c.Runtime.AllowedHosts = strings.Split(v, ",")
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_sampling_interval"))
	if v != "" {
		vDur, err := time.ParseDuration(v)
		if err != nil {
			return errors.Wrapf(err, "runtime_sampling_interval env parse error")
		}
		c.Runtime.SamplingInterval = vDur
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "runtime_watchers"))
	if v != "" {
		watchers := []*WatchConfig{}
		vwatchers := strings.Split(v, ",")
		for _, vw := range vwatchers {
			watchers = append(watchers, &WatchConfig{Type: vw})
		}

		c.Runtime.Watchers = watchers
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "discovery_systemd_glob"))
	if v != "" {
		patterns := []string{}
		vpatterns := strings.Split(v, " ")
		for _, pat := range vpatterns {
			patterns = append(patterns, pat)
		}
		c.Discovery.Systemd.Glob = patterns
	}

	v = os.Getenv(strings.ToUpper(ConfigEnvPrefix + "_" + "discovery_docker_regex"))
	if v != "" {
		patterns := []string{}
		vpatterns := strings.Split(v, ",")
		for _, pat := range vpatterns {
			patterns = append(patterns, pat)
		}
		c.Discovery.Docker.Regex = patterns
	}

	return nil
}

// ensureDefaults ensures sane defaults in global agent configuration
func ensureDefaults(c *AgentConfig) {
	if c.Runtime.SamplingInterval == 0 {
		c.Runtime.SamplingInterval = DefaultRuntimeSamplingInterval
	}

	for _, watchConf := range c.Runtime.Watchers {
		if watchConf.SamplingInterval == 0*time.Second {
			watchConf.SamplingInterval = c.Runtime.SamplingInterval
		}
	}

	if c.Platform.Enabled == nil {
		c.Platform.Enabled = &DefaultPlatformEnabled
	}

	if c.Platform.BatchN == 0 {
		c.Platform.BatchN = DefaultPlatformBatchN
	}

	if c.Platform.MaxPublishInterval == 0 {
		c.Platform.MaxPublishInterval = DefaultPlatformMaxPublishInterval
	}

	if c.Platform.TransportTimeout == 0 {
		c.Platform.TransportTimeout = DefaultPlatformTransportTimeout
	}

	if c.Platform.URI == "" {
		c.Platform.URI = DefaultPlatformURI
	}

	if c.Buffer.MaxHeapAlloc == 0 {
		c.Buffer.MaxHeapAlloc = DefaultBufferMaxHeapAlloc
	}

	if c.Buffer.MinBufferSize == 0 {
		c.Buffer.MinBufferSize = DefaultBufferMinBufferSize
	}

	if c.Buffer.TTL == 0 {
		c.Buffer.TTL = DefaultBufferTTL
	}

	if len(c.Runtime.Log.Outputs) == 0 {
		c.Runtime.Log.Outputs = DefaultRuntimeLoggingOutputs
	}

	if c.Runtime.Log.Lvl == "" {
		c.Runtime.Log.Lvl = DefaultRuntimeLoggingLevel
	}

	if c.Runtime.HTTPAddr == "" {
		c.Runtime.HTTPAddr = DefaultRuntimeHTTPAddr
	}

	if c.Runtime.HostHeaderValidationEnabled == nil {
		c.Runtime.HostHeaderValidationEnabled = &DefaultRuntimeHostHeaderValidationEnabled
	}

	if len(c.Runtime.AllowedHosts) == 0 {
		c.Runtime.AllowedHosts = DefaultRuntimeAllowedHosts
	}

	if c.Runtime.SamplingInterval == 0 {
		c.Runtime.SamplingInterval = DefaultRuntimeSamplingInterval
	}

	if len(c.Runtime.Watchers) == 0 {
		c.Runtime.Watchers = DefaultRuntimeWatchers
	}
}

// userLookup is interface to enable mocking os/user package.
type userLookup interface {
	Current() (*user.User, error)
	LookupGroupID(gid string) (*user.Group, error)
}

type osUserLookup struct {
	*user.User
}

func (o *osUserLookup) Current() (*user.User, error) {
	return user.Current()
}

func (o *osUserLookup) LookupGroupID(gid string) (*user.Group, error) {
	return user.LookupGroupId(gid)
}

var getUserGroupIds = func(u *user.User) ([]string, error) {
	return u.GroupIds()
}

// systemdCanBeActivated returns true if agent user is part of systemd-journal group. Returns false
// and the error if one occurs. Used to deactivate systemd node discovery path.
func systemdCanBeActivated(usr userLookup, targetGrp string) (bool, error) {
	u, err := usr.Current()
	if err != nil {
		return false, err
	}

	gids, err := getUserGroupIds(u)
	if err != nil {
		return false, err
	}

	for _, gid := range gids {
		grp, err := usr.LookupGroupID(gid)
		if err != nil {
			continue
		}
		if grp.Name == targetGrp {
			return true, nil
		}
	}

	return false, nil
}

func clearDeactivatedDiscoveryConfig(c *AgentConfig) {
	if c.Discovery.Docker.Deactivated {
		c.Discovery.Docker.Regex = []string{}
	}

	if c.Discovery.Systemd.Deactivated {
		c.Discovery.Systemd.Glob = []string{}
	}
}

// ensureSystemdActivated force sets discovery.systemd.deactivated to true
// if agent user is not part of systemd-journal group.
func ensureSystemdActivated(c *AgentConfig) error {
	usr := &osUserLookup{}

	act, err := systemdCanBeActivated(usr, "systemd-journal")
	if err != nil {
		return err
	}

	if !act {
		c.Discovery.Systemd.Deactivated = true
	}

	return nil
}

// ensureRequired ensures global agent configuration has loaded required configuration
func ensureRequired(c *AgentConfig) error {
	// Platform variables are only required if Platform Exporter is enabled
	if c.Platform.IsEnabled() {
		if c.Platform.Addr == "" || c.Platform.Addr == PlatformAddrConfigPlaceholder {
			return fmt.Errorf("platform.addr is missing from loaded config")
		}

		if c.Platform.APIKey == "" || c.Platform.APIKey == PlatformAPIKeyConfigPlaceholder {
			return fmt.Errorf("platform.api_key is missing from loaded config")
		}
	}

	return nil
}

// LoadAgentConfig loads agent configuration in the following priority:
// 1. Load configuration from the first file found in ConfigFilePriority.
// 2. Override any configuration key if an environment variable is set.
func LoadAgentConfig(c *AgentConfig) error {
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

	if err := yaml.Unmarshal(content, c); err != nil {
		return err
	}

	if err := overloadFromEnv(c); err != nil {
		return errors.Wrapf(err, "error while loading config from env")
	}

	ensureDefaults(c)

	if err := ensureSystemdActivated(c); err != nil {
		return errors.Wrapf(err, "error checking systemd user group")
	}

	if err := ensureRequired(c); err != nil {
		return errors.Wrapf(err, "loaded configuration is missing a required parameter")
	}

	if err := createLogFolders(c); err != nil {
		return err
	}

	clearDeactivatedDiscoveryConfig(c)

	return nil
}

func createLogFolders(c *AgentConfig) error {
	for _, logPath := range c.Runtime.Log.Outputs {
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

// IsEnabled checks if exporting to the metrika platform is enabled
// in agent's configuration.
// Default: true.
func (p *PlatformConfig) IsEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}
