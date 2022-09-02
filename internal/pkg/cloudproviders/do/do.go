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

package do

import (
	"fmt"
	"net/http"

	"github.com/digitalocean/go-metadata"
)

var client *metadata.Client

func init() {
	client = metadata.NewClient(metadata.WithHTTPClient(&http.Client{Transport: http.DefaultTransport}))
}

func IsRunningOn() bool {
	mt, err := client.Metadata()
	if err != nil {
		return false
	}

	if mt.DropletID == 0 {
		return false
	}

	return true
}

func Hostname() (string, error) {
	mt, err := client.Metadata()
	if err != nil {
		return "", err
	}

	if len(mt.Hostname) == 0 {
		return "", fmt.Errorf("empty hostname")
	}

	return mt.Hostname, nil
}
