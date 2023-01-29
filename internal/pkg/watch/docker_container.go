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
	"strings"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"
)

type discoveryStatus int

const (
	// defaultNodeUpFreq default period for emitting agent.node.up events
	defaultNodeUpFreq = 30 * time.Second

	// defaultNodeDownFreq default period for emitting agent.node.down events
	defaultNodeDownFreq = 5 * time.Second
)

// ContainerWatchConf ContainerWatch configuration struct
type ContainerWatchConf struct {
	RetryIntv            time.Duration
	NodeUpEventFreq      time.Duration
	NodeDownEventFreq    time.Duration
	Discoverer           *utils.NodeDiscoverer
	DockerLogsReaderFunc func(name string) (io.ReadCloser, error)
}

// ContainerWatch uses the docker daemon to monitor the availability of
// a container managed by a discoverer instance. It relies on `docker ls`
// to discover the container and `docker events` to subscribe and receive
// real-time start/stop/restart events of the discovered container. In case
// the container disappears, the watch will infinitely poll the docker daemon
// until the container with the expected name is back up.
type ContainerWatch struct {
	ContainerWatchConf
	Watch

	watchCh      chan interface{}
	stopListCh   chan interface{}
	stopEventsch chan interface{}
}

// ErrContainerWatchConf container watch configuration error
var ErrContainerWatchConf = errors.New("container watch configuration error, missing discoverer?")

// NewContainerWatch ContainerWatch constructor
func NewContainerWatch(conf ContainerWatchConf) (*ContainerWatch, error) {
	w := new(ContainerWatch)
	w.Watch = NewWatch()
	w.ContainerWatchConf = conf
	w.watchCh = make(chan interface{}, 1)
	w.stopListCh = make(chan interface{}, 1)
	w.stopEventsch = make(chan interface{}, 1)

	if conf.NodeDownEventFreq == 0 {
		w.NodeDownEventFreq = defaultNodeDownFreq
	}

	if conf.NodeUpEventFreq == 0 {
		w.NodeUpEventFreq = defaultNodeUpFreq
	}

	if w.RetryIntv == 0 {
		w.RetryIntv = defaultRetryIntv
	}

	if conf.Discoverer == nil {
		return nil, ErrContainerWatchConf
	}

	if w.DockerLogsReaderFunc == nil {
		dockerLogsReaderFunc := func(name string) (io.ReadCloser, error) {
			reader, err := utils.NewDockerLogsReader(name)
			if err != nil {
				return nil, err
			}

			return reader, nil
		}

		w.DockerLogsReaderFunc = dockerLogsReaderFunc
	}
	return w, nil
}

func (w *ContainerWatch) repairEventStream(ctx context.Context) (
	<-chan events.Message, <-chan error, error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	container, err := w.Discoverer.DetectDockerContainer(ctx)
	if err != nil {
		return nil, nil, err
	}

	if container == nil || len(container.Names) == 0 {
		return nil, nil, fmt.Errorf("got nil container or container with empty names, without an error")
	}

	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("status", "start")
	filter.Add("status", "stop")
	filter.Add("status", "kill")
	filter.Add("status", "die")

	// Docker container list api names come with a forward slash
	// which breaks the container filter below if not stripped.
	containerName := strings.TrimPrefix(container.Names[0], "/")
	filter.Add("container", containerName)

	options := dt.EventsOptions{Filters: filter}
	zap.S().Debugw("subscribing to docker event stream", "filter", filter)

	msgchan, errchan, err := utils.DockerEvents(ctx, options)
	if err != nil {
		return nil, nil, err
	}

	reader, err := w.DockerLogsReaderFunc(container.Names[0])
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()

	if err := w.blockchain.ReconfigureByDockerContainer(container, reader); err != nil {
		return nil, nil, err
	}
	global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)

	return msgchan, errchan, nil
}

func (w *ContainerWatch) parseDockerEvent(m events.Message) (string, error) {
	switch m.Status {
	case "start":
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)
		return model.AgentNodeUpName, nil
	case "restart":
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)
		return model.AgentNodeRestartName, nil
	case "die", "stop", "kill":
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
		return model.AgentNodeDownName, nil
	default:
		return "", fmt.Errorf("unknown docker event status: %v", m.Status)
	}
}

// StartUnsafe starts the goroutine for maintaining discovery and
// emitting events about a container's state.
func (w *ContainerWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	var (
		msgchan        <-chan events.Message
		errchan        <-chan error
		ctx            context.Context
		cancel         context.CancelFunc
		err            error
		nodeUpTicker   = time.NewTicker(w.NodeUpEventFreq)
		nodeDownTicker = time.NewTicker(w.NodeDownEventFreq)
	)

	resetTimers := func() {
		nodeDownTicker.Reset(w.NodeDownEventFreq)
		nodeUpTicker.Reset(w.NodeUpEventFreq)
	}
	var sleepd time.Duration
	newEventStream := func() {
		// Retry forever to re-establish the stream. Ensures
		// periodic retries according to the specified interval and
		// probes the stop channel for exit point. Depending on discovery
		// status, agent.node.{up,down} events are emitted.
		for {
			if sleepd != 0 {
				time.Sleep(sleepd)
			} else {
				sleepd = w.RetryIntv
			}

			select {
			case <-w.StopKey:
				return
			default:
			}

			ctx, cancel = context.WithCancel(context.Background())

			w.Log.Debugw("repairing docker event stream")
			if msgchan, errchan, err = w.repairEventStream(ctx); err != nil {
				w.Log.Warnw("getting docker event stream failed", zap.Error(err))
				w.blockchain.SetDockerContainer(nil)
				global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)

				w.emitAgentNodeEvent(model.AgentNodeDownName)
				resetTimers()

				continue
			}

			w.Log.Info("docker event stream ready")
			w.emitAgentNodeEvent(model.AgentNodeUpName)

			resetTimers()

			break
		}
	}

	newEventStream()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		for {
			if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoveryError {
				cancel()
				newEventStream()
			}

			select {
			case m := <-msgchan:
				w.Log.Debugf("docker event message: ID:%s, status:%s, signal:%s",
					m.ID, m.Status, m.Actor.Attributes["signal"])

				evName, err := w.parseDockerEvent(m)
				if err != nil {
					w.Log.Error("error parsing docker event", err)

					continue
				}

				if evName == "" {
					// nothing to do
					continue
				}

				w.emitAgentNodeEvent(evName)
				resetTimers()
			case <-nodeUpTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoveryError {
					// do nothing if node is down

					continue
				}
				w.emitAgentNodeEvent(model.AgentNodeUpName)
			case <-nodeDownTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoverySuccess {
					// do nothing if node is up

					continue
				}
				w.emitAgentNodeEvent(model.AgentNodeDownName)
			case err := <-errchan:
				ctxDone := false
				switch err {
				case context.DeadlineExceeded, context.Canceled, nil:
					ctxDone = true
				}

				if ctxDone {
					continue
				}

				w.Log.Debugf("docker event error: %v, will try to recover the stream", err)
				cancel()

				newEventStream()
			case <-w.StopKey:
				cancel()

				return
			}
		}
	}()
}
