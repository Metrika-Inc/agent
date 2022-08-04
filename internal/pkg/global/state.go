package global

import "sync/atomic"

type platformState int32

var AgentRuntimeState *AgentState

const (
	PlatformStateUnknown platformState = iota
	PlatformStateUp      platformState = iota
	PlatformStateDown    platformState = iota
)

func init() {
	AgentRuntimeState = new(AgentState)
	AgentRuntimeState.Reset()
}

type AgentState struct {
	publishState platformState
}

func (a *AgentState) PublishState() platformState {
	return platformState(atomic.LoadInt32((*int32)(&a.publishState)))
}

func (a *AgentState) SetPublishState(st platformState) {
	atomic.StoreInt32((*int32)(&a.publishState), int32(st))
}

func (a *AgentState) Reset() {
	atomic.StoreInt32((*int32)(&a.publishState), int32(PlatformStateUp))
}
