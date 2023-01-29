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
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	"github.com/docker/docker/api/types"
	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	global.SetBlockchainNode(discover.NewMockBlockchain())
	l, _ := zap.NewProduction()
	zap.ReplaceGlobals(l)
	m.Run()
}

func newMockDockerDaemonHTTP(t *testing.T) *httptest.Server {
	out, err := ioutil.ReadFile("testdata/containers.json")
	require.NoError(t, err)

	handleFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.RequestURI)
		w.Write(out)
	})

	ts := httptest.NewServer(handleFunc)

	return ts
}

func overrideDockerAdapter(url string, mock utils.DockerAdapter) func() {
	defaultDockerAdapterWas := utils.DefaultDockerAdapter
	utils.DefaultDockerAdapter = mock

	return func() {
		utils.DefaultDockerAdapter = defaultDockerAdapterWas
	}
}

type DockerMockAdapterHealthy struct{}

func (d *DockerMockAdapterHealthy) GetRunningContainers() ([]dt.Container, error) {
	return []dt.Container{
		{Names: []string{"/flow-private-network_consensus_3_1"}},
	}, nil
}

func (d *DockerMockAdapterHealthy) MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error) {
	return dt.Container{
		Names: []string{"/flow-private-network_consensus_3_1"},
	}, nil
}

func (d *DockerMockAdapterHealthy) DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	f, err := os.Open("./testdata/docker_logs.json")
	if err != nil {
		return nil, err
	}

	return io.NopCloser(f), nil
}

func (d *DockerMockAdapterHealthy) DockerEvents(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error, error) {
	msgch := make(chan events.Message, 3)
	errch := make(chan error, 1)

	msgch <- events.Message{
		ID:     "100",
		Status: "start",
		Type:   "container",
	}

	msgch <- events.Message{
		ID:     "100",
		Status: "restart",
		Type:   "container",
	}

	msgch <- events.Message{
		ID:     "100",
		Status: "die",
		Type:   "container",
		Actor:  events.Actor{Attributes: map[string]string{"exitCode": "1"}},
	}

	return msgch, errch, nil
}

func TestContainerWatch_happy(t *testing.T) {
	ts := newMockDockerDaemonHTTP(t)
	defer ts.Close()

	mockad := new(DockerMockAdapterHealthy)
	deferme := overrideDockerAdapter(ts.URL, mockad)
	defer deferme()

	dsc, err := utils.NewNodeDiscoverer(utils.NodeDiscovererConfig{ContainerRegex: []string{"consensus_3_1"}})
	require.Nil(t, err)

	w, err := NewContainerWatch(ContainerWatchConf{
		Discoverer: dsc,
	})
	require.Nil(t, err)

	defer w.wg.Wait()
	defer w.Stop()

	emitch := make(chan interface{}, 10)
	w.Subscribe(emitch)

	Start(w)

	expEvents := []string{
		model.AgentNodeUpName,      // emitted on discovery
		model.AgentNodeUpName,      // emitted manually by mock adapter
		model.AgentNodeRestartName, // emitted manually by mock adapter
		model.AgentNodeDownName,    // emitted manually by mock adapter
	}

	for _, ev := range expEvents {
		t.Run(ev, func(t *testing.T) {
			// check agent.node.up event is emitted on discovery
			select {
			case got, ok := <-emitch:
				msg, err := got.(*model.Message)
				require.True(t, err)

				t.Logf("%+v", msg.String())
				require.True(t, ok)
				require.NotNil(t, got)
				require.IsType(t, &model.Message{}, got)
				require.IsType(t, &model.Message_Event{}, msg.Value)
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for event from watch")
			}
		})
	}
}

type DockerMockAdapterError struct {
	once  *sync.Once
	msgch chan events.Message
	errch chan error
}

func (d *DockerMockAdapterError) GetRunningContainers() ([]dt.Container, error) {
	return []dt.Container{
		{Names: []string{"/flow-private-network_consensus_3_1"}},
	}, nil
}

func (d *DockerMockAdapterError) MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error) {
	return dt.Container{
		Names: []string{"/flow-private-network_consensus_3_1"},
	}, nil
}

func (d *DockerMockAdapterError) DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	panic("not implemented")
}

func (d *DockerMockAdapterError) DockerEvents(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error, error) {
	d.once.Do(func() {
		d.msgch = make(chan events.Message, 1)
		d.errch = make(chan error, 1)
		d.errch <- errors.New("mock docker adapter error")

		go func() {
			<-time.After(1 * time.Second)
			d.msgch <- events.Message{
				ID:     "100",
				Status: "restart",
				Type:   "container",
			}
		}()
	})

	return d.msgch, d.errch, nil
}

func TestContainerWatch_error(t *testing.T) {
	ts := newMockDockerDaemonHTTP(t)
	defer ts.Close()

	mockad := &DockerMockAdapterError{once: &sync.Once{}}
	deferme := overrideDockerAdapter(ts.URL, mockad)
	defer deferme()

	dsc, err := utils.NewNodeDiscoverer(utils.NodeDiscovererConfig{ContainerRegex: []string{"consensus_3_1"}})
	require.Nil(t, err)

	w, err := NewContainerWatch(ContainerWatchConf{
		RetryIntv:  10 * time.Millisecond,
		Discoverer: dsc,
		DockerLogsReaderFunc: func(name string) (io.ReadCloser, error) {
			return os.Open("./testdata/docker_logs.json")
		},
	})
	require.Nil(t, err)
	defer w.wg.Wait()
	defer w.Stop()

	emitch := make(chan interface{}, 10)
	w.Subscribe(emitch)

	Start(w)

	expEvents := []string{
		model.AgentNodeUpName,      // emitted on discovery
		model.AgentNodeRestartName, // emitted manually by the mock adapter
	}

	for _, ev := range expEvents {
		t.Run(ev, func(t *testing.T) {
			select {
			case got, ok := <-emitch:
				msg, err := got.(*model.Message)
				require.True(t, err)

				require.True(t, ok)
				require.NotNil(t, got)
				require.IsType(t, &model.Message{}, got)
				require.IsType(t, &model.Message_Event{}, msg.Value)
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for event from watch")
			}
		})
	}
}
