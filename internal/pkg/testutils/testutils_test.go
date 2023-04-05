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

package testutils

import (
	"fmt"
	"os"
	"testing"

	"agent/internal/pkg/global"
	"agent/internal/pkg/watch"
	"agent/internal/pkg/watch/factory"

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
		collectorConfigs = append(collectorConfigs, &global.WatchConfig{Type: w})
	}

	metricFamilies := GetMetricFamilySamples(t, collectorConfigs, outputPath)
	m := jsonpb.Marshaler{}
	file, err := os.OpenFile(outputPath,
		os.O_CREATE|os.O_RDWR, 0o644)
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
		w, err := factory.NewWatcherByType(*collectorConf)
		if err != nil {
			zap.S().Fatalw("watcher factory returned error", "type", collectorConf.Type, zap.Error(err))
		}

		if w == nil {
			zap.S().Fatalw("watcher factory returned nil", "type", collectorConf.Type)
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
