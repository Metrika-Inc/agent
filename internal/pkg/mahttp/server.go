package mahttp

import (
	"net/http"
	"strings"
	"sync"

	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

// ValidationMiddleware is a middleware for validating HTTP requests
// handled by the agent itself.
func ValidationMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hostParts := strings.Split(r.Host, ":")

		if len(hostParts) > 0 {
			host := hostParts[0]
			for _, allowedHost := range global.AgentConf.Runtime.AllowedHosts {
				if allowedHost == host {
					next.ServeHTTP(w, r)
					return
				}
			}

			zap.S().Errorw("got unexpected host header", "host", r.Host)
			w.WriteHeader(http.StatusBadRequest)
		}
	}

	return http.HandlerFunc(fn)
}

// StartHTTPServer starts an HTTP server that listens on addr.
func StartHTTPServer(wg *sync.WaitGroup, addr string) *http.Server {
	s := &http.Server{Addr: addr}

	go func() {
		defer wg.Done()

		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			zap.S().Errorw("ListenAndServe()", zap.Error(err))
		}
	}()

	return s
}
