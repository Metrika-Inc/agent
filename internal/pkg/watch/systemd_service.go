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

	// defaultSystemdDiscoveryTimeout default to wait discovering the systemd unit
	defaultSystemdDiscoveryTimeout = 3 * time.Second
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

	statusTicker := time.NewTicker(w.StatusIntv)

	lastDown := time.Time{}
	lastUp := time.Time{}
	emitNodeDown := func() {
		if time.Since(lastDown) > w.NodeDownEventFreq {
			w.emitAgentNodeEvent(model.AgentNodeDownName)
			lastDown = time.Now()
		}
	}
	emitNodeUp := func() {
		if time.Since(lastUp) > w.NodeUpEventFreq {
			w.emitAgentNodeEvent(model.AgentNodeUpName)
			lastUp = time.Now()
		}
	}
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
				ctx, cancel := context.WithTimeout(context.Background(), defaultSystemdDiscoveryTimeout)
				svc, err := w.Discoverer.DetectSystemdService(ctx)
				cancel()

				dscState := global.AgentRuntimeState.DiscoveryState()

				if err != nil {
					emitNodeDown()
					global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
					w.Log.Errorw("watch error detecting systemd service", zap.Error(err))

					continue
				}

				if svc == nil {
					emitNodeDown()
					global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
					w.Log.Error("got nil systemd service return value")

					continue
				}

				if svc.SubState == "running" {
					if dscState == global.NodeDiscoveryError {
						reader, err := w.JournalReaderFunc(svc.Name)
						if err != nil {
							w.Log.Errorw("error creating journal reader", zap.Error(err))
							continue
						}

						if err := w.blockchain.ReconfigureBySystemdUnit(svc, reader); err != nil {
							if err := reader.Close(); err != nil {
								w.Log.Warnw("error closing journal reader", zap.Error(err))
							}
							w.Log.Errorw("error reconfiguring node from systemd service", zap.Error(err))
							continue
						}

						if err := reader.Close(); err != nil {
							w.Log.Warnw("error closing journal reader", zap.Error(err))
						}
					}
					emitNodeUp()
					global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoverySuccess)
				} else {
					w.Log.Error("systemd unit not in running state", "substate", svc.SubState)
					emitNodeDown()
					global.AgentRuntimeState.SetDiscoveryState(global.NodeDiscoveryError)
				}
			}
		}
	}()
}
