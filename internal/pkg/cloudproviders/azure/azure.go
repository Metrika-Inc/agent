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

package azure

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	metadataReqURL = "http://169.254.169.254/metadata/instance/compute/vmId?api-version=2017-08-01&format=text"
)

// Search implements cloudproviders.Provider interface
type Search struct {
	client  *http.Client
	request *http.Request
}

// NewSearch returns a search object for provider metadata searching.
func NewSearch() *Search {
	req, _ := http.NewRequest("GET", metadataReqURL, nil)
	req.Header.Add("Metadata", "true")
	return &Search{
		client:  &http.Client{Timeout: 15 * time.Second},
		request: req,
	}
}

// IsRunningOn returns true if agent runs on this provider.
func (c *Search) IsRunningOn() bool {
	resp, err := c.client.Do(c.request)
	if err != nil {
		return false
	}

	if resp.StatusCode == http.StatusOK {
		return true
	}

	return err == nil
}

// Hostname returns the hostname of the current instance.
func (c *Search) Hostname() (string, error) {
	resp, err := c.client.Do(c.request)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 response from metadata store")
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

const (
	name = "azure"
)

// Name returns the providers name
func (c *Search) Name() string {
	return name
}
