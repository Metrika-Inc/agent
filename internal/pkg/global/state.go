package global

import "sync/atomic"

type (
	platformState  int32
	discoveryState int32
)

var AgentRuntimeState *AgentState

const (
	PlatformStateUnknown platformState = iota
	PlatformStateUp      platformState = iota
	PlatformStateDown    platformState = iota

	NodeDiscoveryError   discoveryState = iota
	NodeDiscoverySuccess discoveryState = iota
)

func init() {
	AgentRuntimeState = new(AgentState)
	AgentRuntimeState.Reset()
}

type AgentState struct {
	platState platformState
	discState discoveryState
}

func (a *AgentState) PublishState() platformState {
	return platformState(atomic.LoadInt32((*int32)(&a.platState)))
}

func (a *AgentState) SetPublishState(st platformState) {
	atomic.StoreInt32((*int32)(&a.platState), int32(st))
}

func (a *AgentState) DiscoveryState() discoveryState {
	return discoveryState(atomic.LoadInt32((*int32)(&a.discState)))
}

func (a *AgentState) SetDiscoveryState(st discoveryState) {
	atomic.StoreInt32((*int32)(&a.discState), int32(st))
}

func (a *AgentState) Reset() {
	atomic.StoreInt32((*int32)(&a.platState), int32(PlatformStateUp))
	atomic.StoreInt32((*int32)(&a.discState), int32(NodeDiscoveryError))
}
