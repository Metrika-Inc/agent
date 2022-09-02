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

	"go.uber.org/zap"
)

// *** HttpGetWatch ***

type HttpGetWatchConf struct {
	Url      string
	Interval time.Duration
	Headers  map[string]string
	Timeout  time.Duration
}

type HttpGetWatch struct {
	HttpGetWatchConf
	Watch

	client *http.Client

	httpDataCh chan []byte
}

func NewHttpGetWatch(conf HttpGetWatchConf) *HttpGetWatch {
	w := &HttpGetWatch{
		Watch:            NewWatch(),
		HttpGetWatchConf: conf,
		httpDataCh:       make(chan []byte, 10),
	}

	w.Log = w.Log.With("url", w.Url)

	return w
}

func (h *HttpGetWatch) StartUnsafe() {
	h.Watch.StartUnsafe()

	if h.client == nil {
		h.client = &http.Client{}
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		for {
			select {
			case <-time.After(h.Interval):
				ctx, cancel := context.WithTimeout(context.Background(), h.Timeout)
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Url, nil)
				if err != nil {
					h.Log.Errorw("invalid http request", zap.Error(err))
					continue
				}

				for k, v := range h.Headers {
					req.Header.Add(k, v)
				}

				resp, err := h.client.Do(req)
				if err != nil {
					h.Log.Errorw("http request failed", zap.Error(err))
					continue
				}
				if resp.StatusCode > 299 {
					h.Log.Errorw("http request failed", "status_code", resp.StatusCode)
					resp.Body.Close()
					continue
				}

				out, err := io.ReadAll(resp.Body)
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

func (h *HttpGetWatch) Stop() {
	h.Watch.Stop()
}
