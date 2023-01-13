package mahttp

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"agent/internal/pkg/global"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

// TestStartHTTPServer_HostHeaderValidation tests that HTTP servers
// started by StartHTTPServer() and paths wrapped with ValidationMiddleware(),
// validate host header and reject requests for which validation fails.
func TestStartHTTPServer_HostHeaderValidation(t *testing.T) {
	tests := []struct {
		name                        string
		addr                        string
		endpoint                    string
		hostHeader                  string
		allowedHosts                []string
		expCode                     int
		hostHeaderValidationEnabled bool
	}{
		{
			name:                        "valid",
			addr:                        "127.0.0.1:9001",
			endpoint:                    "/metrics",
			allowedHosts:                []string{"127.0.0.1"},
			hostHeader:                  "127.0.0.1",
			hostHeaderValidationEnabled: true,
			expCode:                     200,
		},
		{
			name:                        "invalid host header",
			addr:                        "127.0.0.1:9001",
			endpoint:                    "/metrics",
			allowedHosts:                []string{"127.0.0.1"},
			hostHeader:                  "foobar",
			hostHeaderValidationEnabled: true,
			expCode:                     400,
		},
		{
			name:                        "invalid host header",
			addr:                        "127.0.0.1:9001",
			endpoint:                    "/metrics",
			allowedHosts:                []string{"127.0.0.1"},
			hostHeader:                  "foobar",
			hostHeaderValidationEnabled: false,
			expCode:                     200,
		},
	}

	promHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true})
	http.Handle("/metrics", ValidationMiddleware(promHandler))

	allowedHostsWas := global.AgentConf.Runtime.AllowedHosts
	defer func() { global.AgentConf.Runtime.AllowedHosts = allowedHostsWas }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			global.AgentConf.Runtime.AllowedHosts = tt.allowedHosts
			global.AgentConf.Runtime.HostHeaderValidationEnabled = &tt.hostHeaderValidationEnabled

			wg := &sync.WaitGroup{}
			wg.Add(1)
			srv := StartHTTPServer(wg, tt.addr)
			time.Sleep(1 * time.Second)

			req, err := http.NewRequest("GET", "http://127.0.0.1:9001/metrics", nil)
			require.Nil(t, err)
			req.Host = tt.hostHeader
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			require.Equal(t, tt.expCode, resp.StatusCode)

			httpctx, httpcancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer httpcancel()

			err = srv.Shutdown(httpctx)
			require.Nil(t, err)

			wg.Wait()
		})
	}
}
