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

package equinix

import (
	"github.com/packethost/packngo/metadata"
)

var deviceData *metadata.CurrentDevice

// IsRunningOn returns true if agent runs on Equinix Metal
// ⚠ Layer 3 or Hybrid Bonded networking modes ONLY.
func IsRunningOn() bool {

	var err error

	// save the metadata to deviceData when getting it from the Equinix API,
	// so as to eliminate the second call.
	deviceData, err = metadata.GetMetadata()
	return err == nil && deviceData != nil
}

// Hostname returns the hostname of the current instance.
func Hostname() (string, error) {
	return deviceData.ID, nil
}
