package global

import (
	"agent/pkg/watch"
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v3"
)

var (
	DefaultConfigPath  = "./internal/pkg/global/agent.yml"
	AgentRuntimeConfig AgentConfig
)

type PlatformConfig struct {
	BatchN             int           `yaml:"batch_n"`
	HTTPTimeout        time.Duration `yaml:"http_timeout"`
	MaxPublishInterval time.Duration `yaml:"max_publish_interval"`
	Addr               string        `yaml:"addr"`
	URI                string        `yaml:"uri"`
}

type BufferConfig struct {
	Size uint          `yaml:"size"`
	TTL  time.Duration `yaml:"ttl"`
}

type WatchConfig struct {
	Type             watch.WatchType `yaml:"type"`
	SamplingInterval time.Duration   `yaml:"sampling_interval"`
}

type RuntimeConfig struct {
	MetricsAddr      string         `yaml:"metrics_addr"`
	SamplingInterval time.Duration  `yaml:"sampling_interval"`
	Watchers         []*WatchConfig `yaml:"watchers"`
}

type AgentConfig struct {
	Platform PlatformConfig `yaml:"platform"`
	Buffer   BufferConfig   `yaml:"buffer"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
}

func LoadDefaultConfig() error {
	content, err := ioutil.ReadFile(DefaultConfigPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(content, &AgentRuntimeConfig); err != nil {
		return err
	}

	for _, watchConf := range AgentRuntimeConfig.Runtime.Watchers {
		if watchConf.SamplingInterval == 0*time.Second {
			watchConf.SamplingInterval = AgentRuntimeConfig.Runtime.SamplingInterval
		}
	}

	return nil
}
