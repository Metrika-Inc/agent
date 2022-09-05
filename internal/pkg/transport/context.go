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

package transport

import (
	"context"
	"fmt"

	"agent/internal/pkg/global"
)

var (
	// AgentUUIDContextKey GRPC metadata key for agent UUID.
	AgentUUIDContextKey = "agentUUID"

	// PlatformAddrContextKey context key for platform address.
	PlatformAddrContextKey = "platformAddr"

	// StateContextKey context key for agent state.
	StateContextKey = "agentState"
)

func stateFromContext(ctx context.Context) (*global.AgentState, error) {
	key := StateContextKey
	v, ok := ctx.Value(key).(*global.AgentState)
	if !ok {
		return nil, fmt.Errorf("cannot get %s from ctx %v", AgentUUIDContextKey, ctx)
	}

	return v, nil
}

func stringFromContext(ctx context.Context, key string) (string, error) {
	v, ok := ctx.Value(key).(string)
	if !ok {
		return "", fmt.Errorf("cannot get %s from ctx %v", AgentUUIDContextKey, ctx)
	}

	return v, nil
}
