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
	"io"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/dbus"
	"go.uber.org/zap"
)

const (
	// defaultSystemdStatusIntv default time to wait between status checks
	defaultSystemdStatusIntv = 5 * time.Second
)

// ErrSystemdWatchConf error indicating a watch configuration error
var ErrSystemdWatchConf = errors.New("missing required arguments (regex,discoverer), nothing to watch")

// SystemdServiceWatchConf SystemdServiceWatch configuration struct
type SystemdServiceWatchConf struct {
	StatusIntv        time.Duration
	NodeUpEventFreq   time.Duration
	NodeDownEventFreq time.Duration
	Discoverer        utils.NodeDiscovery
	JournalReaderFunc func(name string) (io.ReadCloser, error)
}

// SystemdServiceWatch uses a dbus connection to monitor the status of
// a systemd service.
type SystemdServiceWatch struct {
	SystemdServiceWatchConf
	Watch

	watchCh  chan interface{}
	dbusConn *dbus.Conn
}

// NewSystemdServiceWatch SystemdServiceWatch constructor
func NewSystemdServiceWatch(conf SystemdServiceWatchConf) (*SystemdServiceWatch, error) {
	w := new(SystemdServiceWatch)
	w.Watch = NewWatch()
	w.SystemdServiceWatchConf = conf
	w.watchCh = make(chan interface{}, 1)

	if conf.NodeDownEventFreq == 0 {
		w.NodeDownEventFreq = defaultNodeDownFreq
	}

	if conf.NodeUpEventFreq == 0 {
		w.NodeUpEventFreq = defaultNodeUpFreq
	}

	if w.StatusIntv == 0 {
		w.StatusIntv = defaultSystemdStatusIntv
	}

	if w.Discoverer == nil {
		return nil, ErrSystemdWatchConf
	}

	if w.JournalReaderFunc == nil {
		journalReaderFunc := func(name string) (io.ReadCloser, error) {
			reader, err := utils.NewJournalReader(w.Discoverer.SystemdService().Name)
			if err != nil {
				return nil, err
			}
			return reader, nil
		}
		w.JournalReaderFunc = journalReaderFunc
	}

	return w, nil
}

// StartUnsafe starts the goroutine for maintaining discovery and
// emitting events about a systemd service.
func (w *SystemdServiceWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	nodeUpTicker := time.NewTicker(w.NodeUpEventFreq)
	nodeDownTicker := time.NewTicker(w.NodeDownEventFreq)
	statusTicker := time.NewTicker(w.StatusIntv)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		for {
			select {
			case <-w.StopKey:
				if w.dbusConn != nil {
					w.dbusConn.Close()
				}

				return
			case <-statusTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				svc, err := w.Discoverer.DetectSystemdService(ctx)
				cancel()

				dscState := global.AgentRuntimeState.DiscoveryState()

				if err != nil {
					w.Log.Errorw("watch error detecting systemd service", zap.Error(err))

					if dscState == global.NodeDiscoverySuccess || dscState == 0 {
						global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
						w.emitAgentNodeEvent(model.AgentNodeDownName)
					}

					continue
				}

				if svc == nil {
					w.Log.Error("got nil systemd service return value")

					if dscState == global.NodeDiscoverySuccess || dscState == 0 {
						global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
						w.emitAgentNodeEvent(model.AgentNodeDownName)
					}

					continue
				}

				if svc.SubState == "running" {
					if dscState == global.NodeDiscoveryError || dscState == 0 {
						reader, err := w.JournalReaderFunc(svc.Name)
						if err != nil {
							w.Log.Errorw("error creating journal reader", zap.Error(err))
							continue
						}

						if err := global.BlockchainNode.ReconfigureBySystemdUnit(svc, reader); err != nil {
							w.Log.Errorw("error reconfiguring node from systemd service", zap.Error(err))
							continue
						}

						if err := reader.Close(); err != nil {
							w.Log.Warnw("error closing journal reader", zap.Error(err))
						}

						global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)
						w.emitAgentNodeEvent(model.AgentNodeUpName)
					}
					// do nothing if node was already up
				} else {
					if dscState == global.NodeDiscoverySuccess || dscState == 0 {
						global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
						w.emitAgentNodeEvent(model.AgentNodeDownName)
					}
					// do nothing if node was already down
				}
			case <-nodeUpTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoverySuccess {
					w.emitAgentNodeEvent(model.AgentNodeUpName)

					continue
				}
				// do nothing if node is down
			case <-nodeDownTicker.C:
				if global.AgentRuntimeState.DiscoveryState() == global.NodeDiscoveryError {
					w.emitAgentNodeEvent(model.AgentNodeDownName)

					continue
				}
				// do nothing if node is up
			}
		}
	}()
}
