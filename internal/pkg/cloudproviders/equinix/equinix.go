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

// Search implements cloudproviders.Provider interface
type Search struct {
	deviceData *metadata.CurrentDevice
}

// NewSearch returns a check object for provider metadata checking.
func NewSearch() *Search {
	return &Search{}
}

// IsRunningOn returns true if agent runs on Equinix Metal
// ⚠ Layer 3 or Hybrid Bonded networking modes ONLY.
func (c *Search) IsRunningOn() bool {
	var err error

	// save the metadata to deviceData when getting it from the Equinix API,
	// so as to eliminate the second call.
	c.deviceData, err = metadata.GetMetadata()
	return err == nil && c.deviceData != nil
}

// Hostname returns the hostname of the current instance.
func (c *Search) Hostname() (string, error) {
	return c.deviceData.ID, nil
}

const (
	name = "equinix"
)

// Name returns the providers name
func (c *Search) Name() string {
	return name
}
