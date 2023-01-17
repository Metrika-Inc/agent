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

package watch

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/global"
	"agent/internal/pkg/mahttp"
	"agent/pkg/timesync"

	"github.com/influxdata/influxdb/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	promDesc        = "Solana InfluxDB metric"
	httpTimeout     = 5 * time.Second
	shutdownTimeout = 5 * time.Second
)

// InfluxExporterWatchConf InfluxExporterWatch configuration struct.
type InfluxExporterWatchConf struct {
	Type    global.WatchType
	Headers map[string]string

	UpstreamURL       *url.URL
	PlatformEnabled   bool
	ExporterActivated bool
	ListenAddr        string
}

// InfluxExporterWatch implements the Watcher interface for collecting
// InfluxDB metrics from data sent to an exposed http endpoint (ListenAddr).
type InfluxExporterWatch struct {
	InfluxExporterWatchConf
	Watch

	httpDataCh   chan writeRequest
	httpwg       *sync.WaitGroup
	srv          *http.Server
	collector    *influxDBCollector
	reverseProxy *httputil.ReverseProxy
	registry     *prometheus.Registry
}

// NewInfluxExporterWatch creates a new influx exporter watch which supports:
// - Reverse proxy Influx DB write API calls to a configurable upstream.
// - Register a prometheus collector to the default registry for exporting
//   metrics collected by the ReverseProxy to Prometheus Exposition Format.
// - Optionally convert collected influx metrics to Openmetrics and using the
//   standard Emit operations for pushing collected data downstream to the
//   agent for publishing to configured agent exporters.
func NewInfluxExporterWatch(conf InfluxExporterWatchConf) *InfluxExporterWatch {
	w := &InfluxExporterWatch{
		Watch:                   NewWatch(),
		InfluxExporterWatchConf: conf,
		httpDataCh:              make(chan writeRequest, 1000),
		httpwg:                  &sync.WaitGroup{},
	}

	ctx := context.Background()
	collector := newInfluxDBCollector(ctx, w.wg)
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	w.registry = registry
	w.collector = collector

	// setup reverse proxy
	if w.UpstreamURL != nil && len(w.UpstreamURL.String()) > 0 {
		w.reverseProxy = newReverseProxy(ctx, w.UpstreamURL)
	}

	return w
}

// StartUnsafe exposes a reverse proxy to an upstream InfluxDB cluster.
// A goroutine is started to consume raw request bodies from /write requests.
// Request body is parsed to prometheus samples and optionally exported in PEF.
func (w *InfluxExporterWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		w.Log.Info("started influx watch")
		for {
			select {
			case <-w.StopKey:
				w.Log.Info("influx watcher stopped")
				return
			case wr := <-w.httpDataCh:
				lastPush.Set(float64(time.Now().UTC().UnixNano()) / 1e9)

				points, err := models.ParsePointsWithPrecision(wr.body, timesync.Now().UTC(), wr.precision)
				if err != nil {
					w.Log.Errorw("error parsing influx line", zap.Error(err))
				}
				if len(points) > 0 {
					w.Log.Infow("influx points parsed", "len", len(points))
				} else {
					continue
				}

				samples := parsePointsToSamples(points)
				w.Log.Debugw("influx samples", "len", len(samples))

				// PEF exposition
				if w.ExporterActivated {
					for i := range samples {
						w.collector.ch <- &samples[i]
					}
				}

				if w.PlatformEnabled {
					// Openmetrics
					metricFamilies := parseSamplesToOpenMetrics(samples)
					w.Log.Infow("parsed metric family", "len", len(metricFamilies))
					for _, family := range metricFamilies {
						msg := &model.Message{
							Name:  "influx." + strings.ToLower(family.Name),
							Value: &model.Message_MetricFamily{MetricFamily: family},
						}
						w.Emit(msg)
					}
				}
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/write", proxyHandler(w.httpDataCh, w.reverseProxy))
	if w.reverseProxy != nil {
		zap.S().Infow("started reverse proxy", "url", w.UpstreamURL)
	}
	mux.HandleFunc("/metrics", promhttp.HandlerFor(w.registry, promhttp.HandlerOpts{}).ServeHTTP)

	w.httpwg.Add(1)
	srv := mahttp.StartHTTPServer(w.httpwg, w.ListenAddr, mux)
	w.srv = srv

	zap.S().Infow("listening for Influx write requests", "addr", w.ListenAddr)
}

// Stop stops the watch.
func (w *InfluxExporterWatch) Stop() {
	// shutdown http server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	w.srv.Shutdown(shutdownCtx)

	w.httpwg.Wait()
	prometheus.Unregister(w.collector)

	// stop processing goroutines
	w.Watch.Stop()
	w.wg.Wait()
}

type writeRequest struct {
	precision string
	body      []byte
}

func proxyHandler(ch chan writeRequest, rproxy *httputil.ReverseProxy) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			// read all bytes from content body and create new stream using it.
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				zap.S().Errorw("error reading body", zap.Error(err))
				w.WriteHeader(http.StatusBadRequest)

				return
			}
			var precision string
			precisionVals, ok := r.URL.Query()["precision"]
			if !ok || len(precisionVals) == 0 {
				zap.S().Warn("precision not specified in query parameters, using default ns")
				precision = "ns"
			} else {
				precision = precisionVals[0]
			}

			ch <- writeRequest{body: body, precision: precision}
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}

		if rproxy != nil {
			rproxy.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func newReverseProxy(ctx context.Context, proxyTargetURL *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(proxyTargetURL)
	proxy.Transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: httpTimeout,
		}).Dial,
		TLSHandshakeTimeout: httpTimeout,
	}

	return proxy
}
