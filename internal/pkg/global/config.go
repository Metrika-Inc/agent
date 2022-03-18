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

	"agent/pkg/watch"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	yaml "gopkg.in/yaml.v3"
)

var (
	// AgentRuntimeConfig the agent runtime configuration
	AgentRuntimeConfig AgentConfig

	// AppName name to use for directories
	AppName = "metrikad"

	// OptPath for agent binary
	OptPath = filepath.Join("/opt", AppName)

	// EtcPath for agent configuration files
	EtcPath = filepath.Join("/etc", AppName)

	// DefaultConfigPath file path to load agent config from
	DefaultConfigPath = filepath.Join(EtcPath, "agent.yml")

	// DefaultFingerprintFilename filename to use for the agent's hostname
	DefaultFingerprintFilename = "fingerprint"

	// AgentCacheDir directory for writing agent runtime data (i.e. hostname)
	AgentCacheDir string

	AgentUUID string
)

func init() {
	var err error

	AgentCacheDir, err = os.UserCacheDir()
	if err != nil {
		zap.S().Fatalw("user cache directory error: ", zap.Error(err))
	}
	AgentCacheDir = filepath.Join(AgentCacheDir, AppName)

	AgentUUID, err = FingerprintSetup()
	if err != nil {
		log.Fatal("fingerprint initialization error: ", err)
	}
}

type PlatformConfig struct {
	BatchN             int           `yaml:"batch_n"`
	TransportTimeout   time.Duration `yaml:"transport_timeout"`
	MaxPublishInterval time.Duration `yaml:"max_publish_interval"`
	Addr               string        `yaml:"addr"`
	URI                string        `yaml:"uri"`
	RetryCount         int           `yaml:"retry_count"`
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
	ReadStream       bool           `yaml:"read_stream"`
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
	EnvFilePath    string   `yaml:"envFile"`
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
	log := zap.S()

	log.Error("trying config: %s", DefaultConfigPath)
	content, err := ioutil.ReadFile(DefaultConfigPath)
	if err != nil {
		metaPath := filepath.Join(EtcPath, DefaultConfigPath)

		log.Error("trying config: %s", metaPath)
		content, err = ioutil.ReadFile(metaPath)
		if err != nil {
			return err
		}

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
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return err
		}
	}

	return nil
}

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
