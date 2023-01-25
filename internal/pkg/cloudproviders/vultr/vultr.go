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

package vultr

import (
	"github.com/vultr/metadata"
)

// Search implements cloudproviders.Provider interface
type Search struct {
	client *metadata.Client
}

// NewSearch returns a check object for provider metadata checking.
func NewSearch() *Search {
	return &Search{client: metadata.NewClient()}
}

// IsRunningOn returns true if agent runs on Vultr.
func (c *Search) IsRunningOn() bool {
	_, err := c.client.Metadata()

	return err == nil
}

// Hostname returns the hostname of the current instance.
func (c *Search) Hostname() (string, error) {
	meta, err := c.client.Metadata()
	if err != nil {
		return "", err
	}

	return meta.InstanceID, nil
}

const (
	name = "vultr"
)

// Name returns the providers name
func (c *Search) Name() string {
	return name
}
