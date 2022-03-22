package global

import (
	"agent/api/v1/model"
	"agent/pkg/fingerprint"
	"agent/pkg/watch"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	WatcherRegistry    WatchersRegisterer
	PrometheusRegistry prometheus.Registerer
	PrometheusGatherer prometheus.Gatherer
	BlockchainNode     Chain
	// Modified at runtime
	Version    = "v0.0.0"
	CommitHash = ""
	Blockchain = "development"
)

type WatchersRegisterer interface {
	Register(w ...watch.Watcher) error
	Start(ch chan<- interface{}) error
	Stop()
	Wait()
}

// Chain provides necessary configuration information
// for the agent core. These methods represent currently
// supported sampler configurations per blockchain protocol basis.
type Chain interface {
	// PEFEndpoints returns a list of HTTP endpoints with PEF data to be sampled.
	PEFEndpoints() []PEFEndpoint

	// ContainerRegex returns a regex-compatible strings to identify the blockchain node
	// if it is running as a docker container.
	ContainerRegex() []string

	// LogEventsList returns a map containing all the blockchain node related events meant to be sampled.
	LogEventsList() map[string]model.FromContext

	// NodeLogPath returns the path to the log file to watch.
	// Supports special keys like "docker" or "journald <service-name>"
	// TODO: string -> []string perhaps
	NodeLogPath() string
}

// PEFEndpoint is a configuration for a single HTTP endpoint
// that exposes metrics in Prometheus Exposition Format.
type PEFEndpoint struct {
	URL     string   `json:"url" yaml:"URL"`
	Filters []string `json:"filters" yaml:"filters"`
}

type DefaultWatcherRegistrar struct {
	watchers []watch.Watcher
}

func (r *DefaultWatcherRegistrar) Register(w ...watch.Watcher) error {
	r.watchers = append(r.watchers, w...)

	return nil
}

func (r *DefaultWatcherRegistrar) Start(ch chan<- interface{}) error {
	for _, w := range r.watchers {
		w.Subscribe(ch)
		watch.Start(w)
	}

	return nil
}

func (r *DefaultWatcherRegistrar) Stop() {
	for _, w := range r.watchers {
		w.Stop()
	}
}

func (r *DefaultWatcherRegistrar) Wait() {
	for _, w := range r.watchers {
		w.Wait()
	}
}

func NewFingerprintWriter(path string) io.WriteCloser {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		zap.S().Fatalw("failed opening a fingerprint file for writing", zap.Error(err))
	}

	return file
}

func NewFingerprintReader(path string) io.ReadCloser {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		zap.S().Fatalw("failed opening fingerprint file for reading", zap.Error(err))
	}

	return file
}

func FingerprintSetup() (string, error) {
	if _, err := os.Stat(AgentCacheDir); errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	fpp := filepath.Join(AgentCacheDir, DefaultFingerprintFilename)
	fpw := NewFingerprintWriter(fpp)
	defer fpw.Close()

	fpr := NewFingerprintReader(fpp)
	defer fpr.Close()

	fp, err := fingerprint.NewWithValidation(fpw, fpr)
	if err != nil {
		if _, ok := err.(*fingerprint.ValidationError); ok {
			return "", fmt.Errorf("cached [%s]: %w", fpp, err)
		}
		return "", err
	}

	if err := fp.Write(); err != nil {
		return "", err
	}

	zap.S().Info("fingerprint ", fp.Hash())

	return fp.Hash(), nil
}

func init() {
	defaultWatcherRegistrar := new(DefaultWatcherRegistrar)
	defaultWatcherRegistrar.watchers = []watch.Watcher{}
	WatcherRegistry = defaultWatcherRegistrar

	PrometheusRegistry = prometheus.NewPedanticRegistry()
	PrometheusGatherer = PrometheusRegistry.(prometheus.Gatherer)
}
