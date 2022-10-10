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

package contrib

import (
	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

// ExportersMap is mapping between an exporter name and its constructor.
// Exporter's constructor takes in one argument of type "any" for its configuration.
// See example: example.go
var ExportersMap = map[string]func(any) (global.Exporter, error){
	"file_stream_exporter": newFileStream,
}

// SetupEnabledExporters takes all exporter-related configurations and constructs
// the relevant exporters. If an exporter constructor returns an error it is logged
// and that exporter is ignored.
// Exporter "A" is deemed enabled if exporterConfigMap["A"] is not nil.
func SetupEnabledExporters(exporterConfigMap map[string]interface{}) []global.Exporter {
	exporters := make([]global.Exporter, 0)
	for expName, exporterCfg := range exporterConfigMap {
		log := zap.S().With("exporter_name", expName)
		var exporter global.Exporter
		var err error

		exporterInitFn, ok := ExportersMap[expName]
		if !ok {
			log.Errorw("unknown exporter specified in config")
			continue
		}

		exporter, err = exporterInitFn(exporterCfg)
		if err != nil {
			log.Errorw("exporter returned an error when initializing", zap.Error(err))
			continue
		}
		exporters = append(exporters, exporter)
	}

	return exporters
}
