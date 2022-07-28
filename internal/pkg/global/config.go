package global

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent/internal/pkg/cloudproviders/do"
	"agent/internal/pkg/cloudproviders/ec2"
	"agent/internal/pkg/cloudproviders/gce"

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

func setAgentHostnameOrFatal() {
	var err error
	if gce.IsRunningOn() {
		// GCE
		AgentHostname, err = gce.Hostname()
	} else if do.IsRunningOn() {
		// Digital Ocean
		AgentHostname, err = do.Hostname()
	} else if ec2.IsRunningOn() {
		// AWS EC2
		AgentHostname, err = ec2.Hostname()
	} else {
		AgentHostname, err = os.Hostname()
	}

	if err != nil {
		log.Fatalf("error getting hostname: %v", err)
	}
}

func init() {
	var err error

	// Agent cache directory (i.e $HOME/.cache/metrikad)
	AgentCacheDir, err = os.UserCacheDir()
	if err != nil {
		log.Fatalf("user cache directory error: :%v", err)
	}

	if err := os.Mkdir(AgentCacheDir, 0o755); err != nil &&
		!errors.Is(err, os.ErrNotExist) && !errors.Is(err, os.ErrExist) {

		log.Fatalf("error creating cache directory: %s (%v)", AgentCacheDir, err)
	}

	// Agent UUID
	setAgentHostnameOrFatal()

	// Fingerprint validation and caching persisted in the cache directory
	_, err = FingerprintSetup()
	if err != nil {
		if !AgentConf.Runtime.DisableFingerprintValidation {
			log.Fatalf("fingerprint initialization error: %v", err)
		}
	}
}

type PlatformConfig struct {
	APIKey             string        `yaml:"api_key"`
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
	Type             string        `yaml:"type"`
	SamplingInterval time.Duration `yaml:"sampling_interval"`
}

type RuntimeConfig struct {
	MetricsAddr                  string         `yaml:"metrics_addr"`
	Log                          LogConfig      `yaml:"logging"`
	SamplingInterval             time.Duration  `yaml:"sampling_interval"`
	ReadStream                   bool           `yaml:"read_stream"`
	Watchers                     []*WatchConfig `yaml:"watchers"`
	DisableFingerprintValidation bool           `yaml:"disable_fingerprint_validation"`
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

type FlowConfig struct {
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

var configFilePriority = []string{
	DefaultAgentConfigName,
	DefaultAgentConfigPath,
}

func LoadDefaultConfig() error {
	var (
		content []byte
		err     error
	)

	for _, fn := range configFilePriority {
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
		return fmt.Errorf("API key is empty. Export env var MA_API_KEY=<yourkey> or use platform.api_key config")
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
