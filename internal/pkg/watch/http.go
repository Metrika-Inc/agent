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
	"context"
	"io"
	"net/http"
	"time"

	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

// *** HttpGetWatch ***

// HTTPWatchConf HttpGetWatch configuration struct.
type HTTPWatchConf struct {
	URL         string
	URLIndex    int
	URLUpdateCh chan global.ConfigUpdate
	Interval    time.Duration
	Headers     map[string]string
	Timeout     time.Duration
}

// HTTPWatch implements the Watcher interface for collecting
// response body from an HTTP endpoint.
type HTTPWatch struct {
	HTTPWatchConf
	Watch

	client *http.Client

	httpDataCh chan []byte
}

// NewHTTPWatch HTTPWatch constructor.
func NewHTTPWatch(conf HTTPWatchConf) *HTTPWatch {
	w := &HTTPWatch{
		Watch:         NewWatch(),
		HTTPWatchConf: conf,
		httpDataCh:    make(chan []byte, 10),
	}

	w.Log = w.Log.With("url", w.URL)

	return w
}

// StartUnsafe starts a goroutine that periodically sends
// GET requests to an HTTP endpoint and emits to the configured
// channel a byte slice of its response body.
func (h *HTTPWatch) StartUnsafe() {
	h.Watch.StartUnsafe()

	if h.client == nil {
		h.client = &http.Client{}
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		for {
			select {
			case ui := <-h.URLUpdateCh:
				eps, ok := ui.Val.([]global.PEFEndpoint)
				if !ok {
					zap.S().Error("type assertion failed for url update")
				}

				if len(eps) == 0 {
					zap.S().Warnw("got empty pef endpoints slice, will ignore", "url", h.URL)
					continue
				}

				if len(eps)+1 <= h.URLIndex {
					zap.S().Warnw("endpoints slice smaller than watch index, will ignore", "len", len(eps), "index", h.URLIndex)

					continue
				}

				// We don't have a way to dynamically schedule HTTP
				// watchers depending on the length of discovered endpoints.
				// Use the watcher's assigned index to pick the corresponding
				// index from the endpoints slice.
				ep := eps[h.URLIndex]

				if ep.URL != h.URL {
					zap.S().Infow("updating HTTP watch URL", "prev", h.URL, "new", ep.URL)
					h.URL = ep.URL
					h.Log = h.Log.With("url", h.URL)
				}
			case <-time.After(h.Interval):
				ctx, cancel := context.WithTimeout(context.Background(), h.Timeout)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
				if err != nil {
					cancel()
					h.Log.Errorw("invalid http request", zap.Error(err))
					continue
				}

				for k, v := range h.Headers {
					req.Header.Add(k, v)
				}

				resp, err := h.client.Do(req)
				if err != nil {
					cancel()
					h.Log.Errorw("http request failed", zap.Error(err))
					continue
				}
				if resp.StatusCode > 299 {
					cancel()
					h.Log.Errorw("http request failed", "status_code", resp.StatusCode)
					resp.Body.Close()
					continue
				}

				out, err := io.ReadAll(resp.Body)
				cancel()
				if err != nil {
					h.Log.Errorw("failed to read PEF body", zap.Error(err))
					resp.Body.Close()
					continue
				}

				if err := resp.Body.Close(); err != nil {
					h.Log.Errorw("failed to close http body", zap.Error(err))
				}

				h.Emit(out)
			case <-h.StopKey:
				return
			}
		}
	}()
}

// Stop stops the watch.
func (h *HTTPWatch) Stop() {
	h.Watch.Stop()
}
