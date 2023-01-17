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
		if !*global.AgentConf.Runtime.HostHeaderValidationEnabled {
			next.ServeHTTP(w, r)
			return
		}

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
func StartHTTPServer(wg *sync.WaitGroup, addr string, mux http.Handler) *http.Server {
	s := &http.Server{Addr: addr, Handler: mux}

	go func() {
		defer wg.Done()

		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			zap.S().Errorw("ListenAndServe()", zap.Error(err))
		}
	}()

	return s
}
