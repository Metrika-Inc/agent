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

// ExportersMap is mapping between an exporter name and its constructor.
// Exporter's constructor takes in one argument of type "any" for its configuration.
// See example: example.go
var ExportersMap = map[string]func(any) (global.Exporter, error){
	"file_stream_exporter": newFileStream,
}
