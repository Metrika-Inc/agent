package global

import (
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v2"
)

var (
	defaultConfigPath  = "./internal/pkg/global/agent.yml"
	AgentRuntimeConfig AgentConfig

	PrometheusNetNetstat WatchType = "prometheus.proc.net.netstat_linux"
	PrometheusNetARP     WatchType = "prometheus.proc.net.arp_linux"
	PrometheusStat       WatchType = "prometheus.proc.stat_linux"
	PrometheusConntrack  WatchType = "prometheus.proc.conntrack_linux"
	PrometheusCPU        WatchType = "prometheus.proc.cpu"
	PrometheusDiskStats  WatchType = "prometheus.proc.diskstats"
	PrometheusEntropy    WatchType = "prometheus.proc.entropy"
	PrometheusFileFD     WatchType = "prometheus.proc.filefd"
	PrometheusFilesystem WatchType = "prometheus.proc.filesystem"
	PrometheusLoadAvg    WatchType = "prometheus.proc.loadavg"
	PrometheusMemInfo    WatchType = "prometheus.proc.meminfo"
	PrometheusNetClass   WatchType = "prometheus.proc.netclass"
	PrometheusNetDev     WatchType = "prometheus.proc.netdev"
	PrometheusSockStat   WatchType = "prometheus.proc.sockstat"
	PrometheusTextfile   WatchType = "prometheus.proc.textfile"
	PrometheusTime       WatchType = "prometheus.time"
	PrometheusUname      WatchType = "prometheus.uname"
	PrometheusVMStat     WatchType = "prometheus.vmstat"
	AlgorandNodeRestart  WatchType = "algorand.node.restart"
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
	MetricsAddr      string         `yaml:"metrics_addr"`
	SamplingInterval time.Duration  `yaml:"sampling_interval"`
	Watchers         []*WatchConfig `yaml:"watchers"`
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

	for _, watchConf := range AgentRuntimeConfig.Runtime.Watchers {
		if watchConf.SamplingInterval == 0*time.Second {
			watchConf.SamplingInterval = AgentRuntimeConfig.Runtime.SamplingInterval
		}
	}
}
