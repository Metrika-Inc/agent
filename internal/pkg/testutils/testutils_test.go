package testutils

import (
	"agent/internal/pkg/factory"
	"agent/internal/pkg/global"
	"agent/pkg/watch"
	"fmt"
	"os"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGenerateCollectorSamples generates a JSONL fixture file
// Where each line is a JSON-serialized dto.MetricFamily type
func TestGenerateCollectorSamples(t *testing.T) {
	outputPath := os.Getenv("AGENT_COLLECTOR_FIXTURE_PATH")
	if outputPath == "" {
		t.Skip()
	}
	collectorList := []string{
		"prometheus.proc.cpu",
		"prometheus.proc.net.netstat_linux",
		"prometheus.proc.net.arp_linux",
		"prometheus.proc.stat_linux",
		"prometheus.proc.conntrack_linux",
		"prometheus.proc.diskstats",
		"prometheus.proc.entropy",
		"prometheus.proc.filefd",
		"prometheus.proc.filesystem",
		"prometheus.proc.loadavg",
		"prometheus.proc.meminfo",
		"prometheus.proc.netclass",
		"prometheus.proc.netdev",
		"prometheus.proc.sockstat",
		"prometheus.proc.textfile",
		"prometheus.time",
		"prometheus.uname",
		"prometheus.vmstat",
	}

	collectorConfigs := []*global.WatchConfig{}
	for _, w := range collectorList {
		collectorConfigs = append(collectorConfigs, &global.WatchConfig{Type: watch.WatchType(w)})
	}

	metricFamilies := GetMetricFamilySamples(t, collectorConfigs, outputPath)
	m := jsonpb.Marshaler{}
	file, err := os.OpenFile(outputPath,
		os.O_CREATE|os.O_RDWR, 0644)
	require.NoError(t, err)
	defer file.Close()
	for _, metricFamily := range metricFamilies {
		m.Marshal(file, metricFamily)
		fmt.Fprint(file, "\n")
	}
}

func GetMetricFamilySamples(t *testing.T, collectorConfigs []*global.WatchConfig, outputPath string) []*dto.MetricFamily {
	watchersEnabled := []*watch.CollectorWatch{}

	for _, collectorConf := range collectorConfigs {
		w := factory.NewWatcherByType(*collectorConf)
		if w == nil {
			zap.S().Fatalf("watcher factory returned nil for type: %v", collectorConf.Type)
		}
		if w, ok := w.(*watch.CollectorWatch); ok {
			watchersEnabled = append(watchersEnabled, w)
			w.StartUnsafe()
		}
	}

	result := []*dto.MetricFamily{}
	for _, watcher := range watchersEnabled {
		mfs, err := watcher.Gatherer.Gather()
		require.NoError(t, err)
		result = append(result, mfs...)

	}

	agentMetrics, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	result = append(result, agentMetrics...)
	return result
}
