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

package ec2

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"go.uber.org/zap"
)

// Search implements cloudproviders.Provider interface
type Search struct {
	client *imds.Client
}

var defaultOptionsFunc = func() imds.Options {
	return imds.Options{}
}

// NewSearch returns a check object for provider metadata checking.
func NewSearch() *Search {
	return &Search{client: imds.New(defaultOptionsFunc())}
}

// IsRunningOn returns true if agent runs on AWS EC2.
func (c *Search) IsRunningOn() bool {
	_, err := c.client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: "instance-id"})

	return err == nil
}

// Hostname returns the hostname of the current instance.
func (c *Search) Hostname() (string, error) {
	resolver := func(key string) (string, error) {
		ih, err := c.client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: key})
		if err != nil {
			return "", err
		}

		b, err := io.ReadAll(ih.Content)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}

	var err error
	var b string

	// it's possible a provider uses EC2 compatible metadata
	// but instance-id is not available.
	keys := []string{"instance-id", "hostname"}
	for _, key := range keys {
		b, err = resolver(key)
		if err != nil {
			zap.S().Warnw("ec2 hostname resolver error", "key", key, zap.Error(err))
			continue
		}

		if len(b) > 0 {
			break
		}
	}

	return b, err
}

const (
	name = "ec2"
)

// Name returns the providers name
func (c *Search) Name() string {
	return name
}
