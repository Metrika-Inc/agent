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
	"io"
	"net/http"
	"time"
)

const (
	metadataReqURL = "http://169.254.169.254/metadata/instance/compute/vmId?api-version=2021-02-01"
)

var client *http.Client

func init() {
	transport := http.DefaultTransport
	transport.(*http.Transport).Proxy = nil
	client = &http.Client{Timeout: 15 * time.Second}
}

// https://learn.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service?tabs=linux#sample-1-tracking-vm-running-on-azure
var newRequest = func() *http.Request {
	req, _ := http.NewRequest("GET", metadataReqURL, nil)
	req.Header.Add("Metadata", "true")

	return req
}

// IsRunningOn returns true if agent runs on Azure.
func IsRunningOn() bool {
	_, err := client.Do(newRequest())

	return err == nil
}

// Hostname returns the hostname of the current instance.
func Hostname() (string, error) {
	resp, err := client.Do(newRequest())
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
