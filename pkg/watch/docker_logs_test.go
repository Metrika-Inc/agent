package watch

import (
	"agent/api/v1/model"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func isOnVoting(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnVoting" {
		return true
	}

	return false
}

type onVoting struct{}

func (o *onVoting) New(v map[string]interface{}) (*model.Event, error) {
	if !isOnVoting(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v,
		"OnVoting",
		"OnVotingTestDesc",
		[]string{"node_role", "node_id", "hotstuff", "chain", "path_id",
			"view", "voted_block_view", "voted_block_id", "voter_id", "time"}...,
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
		Regex: []string{"dapper-private-network_consensus_3_1"},
		Events: map[string]model.FromContext{
			"OnVoting": new(onVoting),
		},
		RetryIntv: 10 * time.Millisecond,
	})
	defer w.Wg.Wait()
	defer w.Stop()

	emitch := make(chan interface{}, 10)
	w.Subscribe(emitch)

	Start(w)

	expMessages := []model.Message{
		{Name: "agent.node.log.found", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
		{Name: "agent.node.log.missing", Type: model.MessageType_event},
		{Name: "agent.node.log.found", Type: model.MessageType_event},
		{Name: "OnVoting", Type: model.MessageType_event},
	}

	for i, expmsg := range expMessages {
		select {
		case got := <-emitch:
			require.NotNil(t, got)
			require.IsType(t, model.Message{}, got)

			gotmsg, _ := got.(model.Message)
			require.Equal(t, expmsg.Type, gotmsg.Type)
			require.Equal(t, expmsg.Name, gotmsg.Name)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for message %s (%d)", expmsg.Name, i)
		}
	}
}
