package global

import (
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v2"
)

var (
	defaultConfigPath  = "./internal/pkg/global/agent.yml"
	AgentRuntimeConfig AgentConfig

	PrometheusNetNetstatLinux WatchType = "prometheus.proc.net.netstat_linux"
	PrometheusNetARPLinux     WatchType = "prometheus.proc.net.arp_linux"
	PrometheusStatLinux       WatchType = "prometheus.proc.stat_linux"
	AlgorandNodeRestart       WatchType = "algorand.node.restart"
)

type WatchType string
type CollectorType WatchType

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
	Type             WatchType     `yaml:"type"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
}

type RuntimeConfig struct {
	MetricsAddr      string        `yaml:"metrics_addr"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
	Watchers         []WatchConfig `yaml:"watchers"`
}

type AgentConfig struct {
	Platform PlatformConfig `yaml:"platform"`
	Buffer   BufferConfig   `yaml:"buffer"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
}

func init() {
	content, err := ioutil.ReadFile(defaultConfigPath)
	if err != nil {
		panic(err)
	}

	if err := yaml.Unmarshal(content, &AgentRuntimeConfig); err != nil {
		panic(err)
	}
}
