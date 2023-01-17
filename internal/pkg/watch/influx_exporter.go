// Copyright 2016 The Prometheus Authors
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

package watch

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"agent/api/v1/model"

	"github.com/influxdata/influxdb/models"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const sampleExpiry = 5 * time.Minute

var lastPush = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "influxdb_last_push_timestamp_seconds",
		Help: "Unix timestamp of the last received influxdb metrics push in seconds.",
	},
)

type influxDBCollector struct {
	samples map[string]*influxDBSample
	mu      sync.Mutex
	ch      chan *influxDBSample
}

func newInfluxDBCollector(ctx context.Context, wg *sync.WaitGroup) *influxDBCollector {
	c := &influxDBCollector{
		ch:      make(chan *influxDBSample, 1000),
		samples: map[string]*influxDBSample{},
	}
	go c.processSamples(ctx, wg)
	return c
}

func (c *influxDBCollector) processSamples(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(time.Minute).C
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-c.ch:
			c.mu.Lock()
			prev, ok := c.samples[s.ID]
			if !ok {
				c.samples[s.ID] = s
				c.mu.Unlock()
				continue
			}

			if s.Timestamp.After(prev.Timestamp) {
				c.samples[s.ID] = s
			}

			c.mu.Unlock()
		case <-ticker:
			// Garbage collect expired value lists.
			ageLimit := time.Now().Add(-sampleExpiry)
			c.mu.Lock()
			for k, sample := range c.samples {
				if ageLimit.After(sample.Timestamp) {
					delete(c.samples, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

// Collect implements prometheus.Collector.
func (c *influxDBCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- lastPush

	c.mu.Lock()
	samples := make([]*influxDBSample, 0, len(c.samples))
	for _, sample := range c.samples {
		samples = append(samples, sample)
	}
	c.mu.Unlock()

	ageLimit := time.Now().Add(-sampleExpiry)
	for _, sample := range samples {
		if ageLimit.After(sample.Timestamp) {
			continue
		}

		metric := prometheus.NewMetricWithTimestamp(
			sample.Timestamp,
			prometheus.MustNewConstMetric(
				prometheus.NewDesc(sample.Name, promDesc, []string{}, sample.Labels),
				prometheus.UntypedValue,
				sample.Value,
			),
		)
		ch <- metric
	}
}

// Describe implements prometheus.Collector.
func (c *influxDBCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lastPush.Desc()
}

type influxDBSample struct {
	ID        string
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

// ReplaceInvalidChars analog of invalidChars = regexp.MustCompile("[^a-zA-Z0-9_]")
func ReplaceInvalidChars(in *string) {
	for charIndex, char := range *in {
		charInt := int(char)
		if !((charInt >= 97 && charInt <= 122) || // a-z
			(charInt >= 65 && charInt <= 90) || // A-Z
			(charInt >= 48 && charInt <= 57) || // 0-9
			charInt == 95) { // _

			*in = (*in)[:charIndex] + "_" + (*in)[charIndex+1:]
		}
	}
	// prefix with _ if first char is 0-9
	if int((*in)[0]) >= 48 && int((*in)[0]) <= 57 {
		*in = "_" + *in
	}
}

func parsePointsToSamples(points []models.Point) []influxDBSample {
	samples := []influxDBSample{}
	for _, s := range points {
		fields, err := s.Fields()
		if err != nil {
			zap.S().Errorw("error getting fields from point", zap.Error(err))

			continue
		}

		for field, v := range fields {
			var value float64
			switch v := v.(type) {
			case float64:
				value = v
			case int64:
				value = float64(v)
			case bool:
				if v {
					value = 1
				} else {
					value = 0
				}
			default:
				continue
			}

			var name string
			if field == "value" {
				name = string(s.Name())
			} else {
				name = string(s.Name()) + "_" + field
			}

			ReplaceInvalidChars(&name)
			sample := influxDBSample{
				Name:      name,
				Timestamp: s.Time(),
				Value:     value,
				Labels:    map[string]string{},
			}
			for _, v := range s.Tags() {
				key := string(v.Key)
				if key == "__name__" {
					continue
				}
				ReplaceInvalidChars(&key)
				sample.Labels[key] = string(v.Value)
			}

			// Calculate a consistent unique ID for the sample.
			labelnames := make([]string, 0, len(sample.Labels))
			for k := range sample.Labels {
				labelnames = append(labelnames, k)
			}
			sort.Strings(labelnames)
			parts := make([]string, 0, len(sample.Labels)*2+1)
			parts = append(parts, name)
			for _, l := range labelnames {
				parts = append(parts, l, sample.Labels[l])
			}
			sample.ID = strings.Join(parts, ".")
			samples = append(samples, sample)
		}
	}
	return samples
}

func parseSamplesToOpenMetrics(samples []influxDBSample) []*model.MetricFamily {
	families := make([]*model.MetricFamily, len(samples))

	for i, smpl := range samples {
		mf := &model.MetricFamily{}
		mf.Name = smpl.Name
		mf.Type = model.MetricType_UNKNOWN

		metric := &model.Metric{}
		metric.Labels = make([]*model.Label, len(smpl.Labels))
		lblIdx := 0
		for k, v := range smpl.Labels {
			lbl := &model.Label{Name: k, Value: v}
			metric.Labels[lblIdx] = lbl
			lblIdx++
		}
		metric.MetricPoints = []*model.MetricPoint{
			{
				Timestamp: timestamppb.New(smpl.Timestamp),
				Value: &model.MetricPoint_UnknownValue{
					UnknownValue: &model.UnknownValue{
						Value: &model.UnknownValue_DoubleValue{
							DoubleValue: smpl.Value,
						},
					},
				},
			},
		}
		mf.Metrics = []*model.Metric{metric}
		families[i] = mf
	}
	return families
}
