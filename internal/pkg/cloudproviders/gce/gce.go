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

package gce

import (
	"net/http"

	"cloud.google.com/go/compute/metadata"
)

// Search implements cloudproviders.Provider interface
type Search struct {
	client *metadata.Client
}

// NewSearch returns a check object for provider metadata checking.
func NewSearch() *Search {
	return &Search{client: metadata.NewClient(&http.Client{Transport: http.DefaultTransport})}
}

// IsRunningOn returns true if agent runs on GCE.
func (c *Search) IsRunningOn() bool {
	return metadata.OnGCE()
}

// Hostname returns the hostname of the current instance.
func (c *Search) Hostname() (string, error) {
	return c.client.Hostname()
}

const (
	name = "gce"
)

// Name returns the providers name
func (c *Search) Name() string {
	return name
}
