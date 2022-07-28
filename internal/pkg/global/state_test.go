package global

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentState(t *testing.T) {
	// check default global state is initialized
	require.NotNil(t, AgentRuntimeState)

	testState := new(AgentState)

	// check default state
	require.Equal(t, PlatformStateUknown, testState.PublishState())

	testState.SetPublishState(PlatformStateUp)
	require.Equal(t, PlatformStateUp, testState.PublishState())
}
