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
)

var ExportersMap = map[string]func(any) (global.Exporter, error){
	"file_stream_exporter": newFileStream,
}

// // GetExporters get enabled exporters.
// func GetExporters() ([]global.Exporter, error) {
// 	exporters := []global.Exporter{}
// 	for name, exporterSetup := range enabledExporters {
// 		exporter, err := exporterSetup()
// 		if err != nil {
// 			zap.S().Errorw("failed initializing exporter", "exporter_name", name)
// 			return nil, err
// 		}
// 		exporters = append(exporters, exporter)
// 	}

// 	return exporters, nil
// }
