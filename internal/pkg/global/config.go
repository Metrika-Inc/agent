package global

import (
	"agent/pkg/watch"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
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

	if err := createLogFolders(); err != nil {
		return err
	}

	return nil
}

func createLogFolders() error {
	for _, logPath := range AgentRuntimeConfig.Runtime.Log.Outputs {
		if strings.HasSuffix(logPath, "/") {
			return fmt.Errorf("invalid log output path ending with '/': %s", logPath)
		}
		pathSplit := strings.Split(logPath, "/")
		if len(pathSplit) == 1 {
			continue
		}
		folder := strings.Join(pathSplit[:len(pathSplit)-1], "/")
		if err := os.MkdirAll(folder, 0755); err != nil {
			return err
		}
	}

	return nil
}