package watch

import (
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/flow"
	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
)

func isOnVoting(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnVoting" {
		return true
	}

	return false
}

type onVoting struct{}

func (o *onVoting) New(v map[string]interface{}, t time.Time) (*model.Event, error) {
	if !isOnVoting(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v,
		"OnVoting",
		t,
		[]string{
			"node_role", "node_id", "hotstuff", "chain", "path_id",
			"view", "voted_block_view", "voted_block_id", "voter_id", "time",
		}...,
	)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func TestDockerLogs_happy(t *testing.T) {
	ts := newMockDockerDaemonHTTP(t)
	defer ts.Close()

	mockad := new(DockerMockAdapterHealthy)
	deferme := overrideDockerAdapter(ts.URL, mockad)
	defer deferme()

	w := NewDockerLogWatch(DockerLogWatchConf{
		Regex: []string{"flow-private-network_consensus_3_1"},
		Events: map[string]model.FromContext{
			"OnVoting": new(onVoting),
		},
		RetryIntv: 10 * time.Millisecond,
	})
	defer w.wg.Wait()
	defer w.Stop()

	emitch := make(chan interface{}, 10)
	w.Subscribe(emitch)
	global.BlockchainNode = &flow.Flow{}
	Start(w)

	expMessages := []*model.Message{
		{Name: "agent.node.log.found"},
		{Name: "OnVoting"},
		{Name: "OnVoting"},
		{Name: "OnVoting"},
		{Name: "OnVoting"},
		{Name: "OnVoting"},
		{Name: "agent.node.log.missing"},
		{Name: "agent.node.log.found"},
		{Name: "OnVoting"},
	}

	for i, expmsg := range expMessages {
		select {
		case got := <-emitch:
			require.NotNil(t, got)
			require.IsType(t, &model.Message{}, got)

			gotmsg, ok := got.(*model.Message)
			require.True(t, ok)
			require.Equal(t, expmsg.Name, gotmsg.Name)
			require.IsType(t, &model.Message_Event{}, gotmsg.Value)
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for message %s (%d)", expmsg.Name, i)
		}
	}
}
