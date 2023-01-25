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

package collector

import (
	"fmt"
	"os"
	"testing"

	"agent/internal/pkg/global"
)

func TestMain(m *testing.M) {
	os.Setenv("MA_PLATFORM", "agent.test.metrika.co:443")
	os.Setenv("MA_API_KEY", "test_api_key")
	os.Setenv("MA_DISCOVERY_HINTS_DOCKER", "test-discovery-name")
	if err := global.LoadAgentConfig(); err != nil {
		fmt.Printf("failed to load config: %v\n", err)

		os.Exit(1)
	}

	os.Exit(m.Run())
}
