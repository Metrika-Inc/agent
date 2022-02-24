package watch

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"
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
	wg     *sync.WaitGroup

	httpDataCh chan []byte
}

func NewHttpGetWatch(conf HttpGetWatchConf) *HttpGetWatch {
	w := &HttpGetWatch{
		Watch:            NewWatch(),
		HttpGetWatchConf: conf,
		httpDataCh:       make(chan []byte, 10),
	}

	w.Log = w.Log.With("url", w.Url)

	w.wg = new(sync.WaitGroup)
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
			case <-time.After(5 * time.Second):
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
					continue
				}

				out, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					h.Log.Errorw("failed to read PEF body", zap.Error(err))
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
	h.wg.Wait()
}