package flow

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	"github.com/docker/docker/api/types"
	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
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

type DockerMockAdapterHealthy struct {
	logFile string
}

func (d *DockerMockAdapterHealthy) Close() error {
	return nil
}

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
	if d.logFile == "" {
		d.logFile = "./testdata/docker_logs.json"
	}

	f, err := os.Open(d.logFile)
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

func TestDiscoverContainer_Network_NodeRole(t *testing.T) {
	tests := []struct {
		name         string
		testdataFile string
		expNetwork   string
		expNodeRole  string
		expErr       bool
	}{
		{
			name:         "network by chain_id",
			testdataFile: "./testdata/networkExtraction-1.json",
			expNetwork:   "flow-localnet",
			expNodeRole:  "verification",
		},
		{
			name:         "network by chain",
			testdataFile: "./testdata/networkExtraction-2.json",
			expNetwork:   "flow-mainnet",
			expNodeRole:  "verification",
		},
		{
			name:         "missing network and node role",
			testdataFile: "./testdata/networkExtraction-3.json",
			expNetwork:   "",
			expNodeRole:  "",
			expErr:       true,
		},
	}

	ts := newMockDockerDaemonHTTP(t)
	defer ts.Close()
	mockad := new(DockerMockAdapterHealthy)
	deferme := overrideDockerAdapter(ts.URL, mockad)
	defer deferme()

	defaultFlowConfigPathWas := DefaultFlowPath
	DefaultFlowPath = "./testdata/flow-config.yml"
	defer func() {
		DefaultFlowPath = defaultFlowConfigPathWas
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockad.logFile = tt.testdataFile

			flow, err := NewFlow()
			require.Nil(t, err)
			require.NotNil(t, flow)

			dsc, err := utils.NewNodeDiscoverer(utils.NodeDiscovererConfig{ContainerRegex: []string{"consensus_3_1"}})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			container, err := dsc.DetectDockerContainer(ctx)
			require.Nil(t, err)
			cancel()

			reader, err := utils.NewDockerLogsReader(container.Names[0])
			require.Nil(t, err)
			flow.SetRunScheme(global.NodeDocker)
			err = flow.ReconfigureByDockerContainer(container, reader)
			if tt.expErr {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
				require.NotNil(t, container)

				require.Equal(t, tt.expNetwork, flow.network)
				require.Equal(t, tt.expNodeRole, flow.nodeRole)

			}
		})
	}
}
