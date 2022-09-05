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

package global

import "sync/atomic"

type (
	platformState  int32
	discoveryState int32
)

// AgentRuntimeState agent state as determined by the agent.
var AgentRuntimeState *AgentState

const (
	// PlatformStateUnknown platform publishing state is uknown.
	PlatformStateUnknown int32 = iota

	// PlatformStateUp platform publishing state is healthy.
	PlatformStateUp int32 = iota

	// PlatformStateDown platform publishing state is unhealthy.
	PlatformStateDown int32 = iota

	// NodeDiscoveryError there was an error during the node discovery process.
	NodeDiscoveryError int32 = iota

	// NodeDiscoverySuccess node discovery succesfull.
	NodeDiscoverySuccess int32 = iota
)

func init() {
	AgentRuntimeState = new(AgentState)
	AgentRuntimeState.Reset()
}

// AgentState maintains available state.
type AgentState struct {
	platState int32
	discState int32
}

// PublishState returns current platform publish state.
func (a *AgentState) PublishState() int32 {
	return atomic.LoadInt32((*int32)(&a.platState))
}

// SetPublishState sets the platform publish state.
func (a *AgentState) SetPublishState(st int32) {
	atomic.StoreInt32((*int32)(&a.platState), int32(st))
}

// DiscoveryState returns the current node discovery state.
func (a *AgentState) DiscoveryState() int32 {
	return atomic.LoadInt32((*int32)(&a.discState))
}

// SetDiscoveryState sets node discovery state.
func (a *AgentState) SetDiscoveryState(st int32) {
	atomic.StoreInt32((*int32)(&a.discState), int32(st))
}

// Reset sets default values for all maintained state values.
func (a *AgentState) Reset() {
	atomic.StoreInt32((*int32)(&a.platState), PlatformStateUp)
	atomic.StoreInt32((*int32)(&a.discState), NodeDiscoveryError)
}
