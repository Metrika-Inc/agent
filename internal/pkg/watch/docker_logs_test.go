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

package watch

import (
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover"
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

func TestDockerLogs_disabled(t *testing.T) {
	// ensure we're configuring mock blockchain to not watch logs for this test
	disableDockerLogWatch(t)
	defer enableDockerLogWatch(t)

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

	Start(w)

	<-time.After(50 * time.Millisecond)

	_, ok := <-w.StopKey
	require.False(t, ok)
	require.Len(t, emitch, 0)
}

func disableDockerLogWatch(t *testing.T) {
	mockChain, ok := global.BlockchainNode.(*discover.MockBlockchain)
	require.True(t, ok)
	mockChain.SetLogWatchEnabled(false)
}

func enableDockerLogWatch(t *testing.T) {
	mockChain, ok := global.BlockchainNode.(*discover.MockBlockchain)
	require.True(t, ok)
	mockChain.SetLogWatchEnabled(true)
}
