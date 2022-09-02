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
)

var client *imds.Client

func init() {
	client = imds.New(imds.Options{})
}

func IsRunningOn() bool {
	_, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: "instance-id",
	})

	return err == nil
}

func Hostname() (string, error) {

	// Using 'internal-hostname' as the 'public-hostname' may not exist!
	ih, err := client.GetMetadata(context.TODO(),
		&imds.GetMetadataInput{
			Path: "instance-id"})

	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(ih.Content)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
