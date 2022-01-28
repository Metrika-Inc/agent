package global

import (
	"agent/pkg/watch"
	"io/ioutil"
	"time"

	"go.uber.org/zap/zapcore"
	yaml "gopkg.in/yaml.v3"
)

var (
	DefaultConfigPath  = "./internal/pkg/global/agent.yml"
	DefaultDapperPath  = "./internal/pkg/global/dapper.yml"
	DefaultAlgoPath    = "./internal/pkg/global/algorand.yml"
	AgentRuntimeConfig AgentConfig
	DapperConf         *DapperConfig
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
	Log              LogConfig      `yaml:"logging"`
	SamplingInterval time.Duration  `yaml:"sampling_interval"`
	Watchers         []*WatchConfig `yaml:"watchers"`
}

type AgentConfig struct {
	Platform PlatformConfig `yaml:"platform"`
	Buffer   BufferConfig   `yaml:"buffer"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
}

type LogConfig struct {
	Lvl     string   `yaml:"level"`
	Outputs []string `yaml:"outputs"`
}

type DapperConfig struct {
	Client         string   `yaml:"client"`
	ContainerRegex []string `yaml:"containerRegex"`
	NodeID         string   `yaml:"nodeID"`
	PEFEndpoints   []string `yaml:"pefEndpoints"`
}

var zapLevelMapper = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

func (l LogConfig) Level() zapcore.Level {
	return zapLevelMapper[l.Lvl]
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

func LoadDapperConfig() error {
	var c DapperConfig
	content, err := ioutil.ReadFile(DefaultDapperPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(content, &c); err != nil {
		return err
	}
	DapperConf = &c
	return nil
}

func (d *DapperConfig) Default() *DapperConfig {
	return &DapperConfig{
		Client: "flow-go",
		ContainerRegex: []string{
			"flow-go",
		},
		PEFEndpoints: []string{
			"127.0.0.1:8080/metrics",
		},
	}
}
