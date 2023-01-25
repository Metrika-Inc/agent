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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	client     *http.Client
	request, _ = http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
)

// Compute compute section in metadata response.
type Compute struct {
	VMID string `json:"vmId"`
}

// Metadata struct to hold unmarshal metadata response.
type Metadata struct {
	Compute Compute `json:"compute"`
}

func init() {
	transport := http.DefaultTransport
	transport.(*http.Transport).Proxy = nil
	client = &http.Client{Timeout: 15 * time.Second}
}

// IsRunningOn returns true if agent runs on Azure.
func IsRunningOn() bool {
	_, err := client.Do(request)

	return err == nil
}

// Hostname returns the hostname of the current instance.
func Hostname() (string, error) {
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var md Metadata
	if err := json.Unmarshal(b, &md); err != nil {
		return "", err
	}

	if md.Compute.VMID == "" {
		return "", fmt.Errorf("got empty compute.vmId from metadata store")
	}

	return md.Compute.VMID, nil
}
