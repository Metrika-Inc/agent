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
	"fmt"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

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

// newNodeEventCtx creates a map with the context for agent.node.* events.
// Properties with empty values are omitted.
func newNodeEventCtx() map[string]interface{} {
	ctx := map[string]interface{}{}

	nodeID := global.BlockchainNode.NodeID()
	if nodeID != "" {
		ctx[model.NodeIDKey] = nodeID
	}

	nodeType := global.BlockchainNode.NodeType()
	if nodeType != "" {
		ctx[model.NodeTypeKey] = nodeType
	}

	nodeVersion := global.BlockchainNode.NodeVersion()
	if nodeVersion != "" {
		ctx[model.NodeVersionKey] = nodeVersion
	}

	network := global.BlockchainNode.Network()
	if network != "" {
		ctx[model.NetworkKey] = network
	}

	return ctx
}

// ContainerWatchConf ContainerWatch configuration struct
type ContainerWatchConf struct {
	Regex             []string
	RetryIntv         time.Duration
	NodeUpEventFreq   time.Duration
	NodeDownEventFreq time.Duration
}

// ContainerWatch uses the docker daemon to monitor the availability of
// a container matching a given regex. It relies on `docker ls` to discover
// the container and `docker events` to subscribe and receive
// real-time start/stop/restart events of the discovered container. In case
// the container disappears, the watch will infinitely poll the docker daemon
// until the container with the expected name is back up.
type ContainerWatch struct {
	ContainerWatchConf
	Watch

	watchCh       chan interface{}
	stopListCh    chan interface{}
	stopEventsch  chan interface{}
	containerGone bool
}

// NewContainerWatch ContainerWatch constructor
func NewContainerWatch(conf ContainerWatchConf) *ContainerWatch {
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

	return w
}

func (w *ContainerWatch) repairEventStream(ctx context.Context) (
	<-chan events.Message, <-chan error, error,
) {
	container, err := global.BlockchainNode.DiscoverContainer()
	if err != nil {
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)

		return nil, nil, err
	}

	if container == nil {
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)

		return nil, nil, fmt.Errorf("got nil container without an error")
	}

	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("status", "start")
	filter.Add("status", "stop")
	filter.Add("status", "kill")
	filter.Add("status", "die")
	filter.Add("container", container.ID)
	options := dt.EventsOptions{Filters: filter}

	msgchan, errchan, err := utils.DockerEvents(ctx, options)
	if err != nil {
		global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)

		return nil, nil, err
	}
	global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)

	return msgchan, errchan, nil
}

func (w *ContainerWatch) parseDockerEvent(m events.Message) (*model.Event, error) {
	var ev *model.Event
	var err error
	ctx := map[string]interface{}{
		model.NodeIDKey:      global.BlockchainNode.NodeID(),
		model.NodeTypeKey:    global.BlockchainNode.NodeType(),
		model.NodeVersionKey: global.BlockchainNode.NodeVersion(),
	}

	switch m.Status {
	case "start":
		ev, err = model.NewWithCtx(ctx, model.AgentNodeUpName, timesync.Now())
		w.containerGone = false
	case "restart":
		ev, err = model.NewWithCtx(ctx, model.AgentNodeRestartName, timesync.Now())
		w.containerGone = false
	case "die":
		exitCode, ok := m.Actor.Attributes["exit_code"]
		if ok {
			ctx["exit_code"] = exitCode
		}

		ev, err = model.NewWithCtx(ctx, model.AgentNodeDownName, timesync.Now())
		w.containerGone = true
	}

	if err != nil {
		return nil, err
	}

	return ev, nil
}

// StartUnsafe starts the goroutine for maintaining discovery and
// emitting events about a container's state.
func (w *ContainerWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	if len(w.Regex) < 1 {
		w.Log.Error("missing required argument 'regex', nothing to watch")

		return
	}

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

			var (
				ev    *model.Event
				evCtx = newNodeEventCtx()
				evErr error
			)
			ctx, cancel = context.WithCancel(context.Background())

			w.Log.Debugw("repairing docker event stream")
			if msgchan, errchan, err = w.repairEventStream(ctx); err != nil {
				w.Log.Warnw("getting docker event stream failed", zap.Error(err))
				w.containerGone = true

				ev, evErr = model.NewWithCtx(evCtx, model.AgentNodeDownName, timesync.Now())
				if evErr != nil {
					zap.S().Errorw("error creating event", zap.Error(err))

					continue
				}

				if err := emit.Ev(w, ev); err != nil {
					zap.S().Errorw("error emitting event", zap.Error(err))
				} else {
					resetTimers()
				}

				continue
			}

			w.Log.Info("docker event stream ready")
			w.containerGone = false

			ev, evErr = model.NewWithCtx(evCtx, model.AgentNodeUpName, timesync.Now())
			if evErr != nil {
				zap.S().Errorw("error creating event", zap.Error(err))

				continue
			}

			if err := emit.Ev(w, ev); err != nil {
				zap.S().Errorw("error emitting event", zap.Error(err))
			} else {
				resetTimers()
			}

			break
		}
	}

	newEventStream()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		for {
			if w.containerGone {
				cancel()
				newEventStream()
			}

			select {
			case m := <-msgchan:
				w.Log.Debugf("docker event message: ID:%s, status:%s, signal:%s",
					m.ID, m.Status, m.Actor.Attributes["signal"])

				ev, err := w.parseDockerEvent(m)
				if err != nil {
					w.Log.Error("error parsing docker event", err)

					continue
				}

				if ev == nil {
					// nothing to do
					continue
				}

				if err := emit.Ev(w, ev); err != nil {
					w.Log.Error("error emitting docker event", err)
				} else {
					resetTimers()
				}
			case <-nodeUpTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoveryError {
					// do nothing if node is down

					continue
				}

				ctx := newNodeEventCtx()
				ev, err := model.NewWithCtx(ctx, model.AgentNodeUpName, timesync.Now())
				if err != nil {
					w.Log.Errorw("failed to create node up event", zap.Error(err))
					continue
				}

				if err := emit.Ev(w, ev); err != nil {
					zap.S().Errorw("error emitting node up event", zap.Error(err))
					continue
				}
			case <-nodeDownTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoverySuccess {
					// do nothing if node is up

					continue
				}

				ctx := newNodeEventCtx()
				ev, err := model.NewWithCtx(ctx, model.AgentNodeDownName, timesync.Now())
				if err != nil {
					w.Log.Errorw("failed to create node down event", zap.Error(err))
					continue
				}

				if err := emit.Ev(w, ev); err != nil {
					zap.S().Errorw("error emitting node down event", zap.Error(err))
					continue
				}
			case err := <-errchan:
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
