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

package transport

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	platformPublishErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_platform_publish_errors_total", Help: "The total number of errors while sending data to the platform.",
	})

	metricsPublishedCnt = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_metrics_published_total_count", Help: "The total number of metrics successfully published",
	})
)
