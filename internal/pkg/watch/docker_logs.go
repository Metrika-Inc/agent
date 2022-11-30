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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const maxLineBytes = uint32(1024 * 1024)

var defaultRetryIntv = 3 * time.Second

// DockerLogWatchConf DockerLogWatch configuration struct.
type DockerLogWatchConf struct {
	Regex     []string
	Events    map[string]model.FromContext
	RetryIntv time.Duration
}

// DockerLogWatch uses the host docker daemon to discover a
// container's log file based on list of regexes matched against
// the container name. If discovery fails, watch will periodically
// retry forever every RetryIntv until the container is back.
type DockerLogWatch struct {
	DockerLogWatchConf
	Watch

	rc io.ReadCloser
}

// NewDockerLogWatch DockerLogWatch constructor
func NewDockerLogWatch(conf DockerLogWatchConf) *DockerLogWatch {
	w := new(DockerLogWatch)
	w.DockerLogWatchConf = conf
	w.Watch = NewWatch()
	w.Log = w.Log.With("watch", "docker_logs")
	if w.RetryIntv == 0 {
		w.RetryIntv = defaultRetryIntv
	}

	return w
}

func (w *DockerLogWatch) repairLogStream(ctx context.Context) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "0",
	}

	var rc io.ReadCloser
	var err error
	for _, regex := range w.Regex {
		rc, err = utils.DockerLogs(ctx, regex, options)
		if err == nil {
			// successfully matched with container, exit early
			return rc, nil
		}

	}
	zap.S().Errorw("failed getting docker logs", "container_regex_list", w.Regex, "last_error", err)
	return nil, fmt.Errorf("failed repairing the log stream, last error: %w", err)
}

func (w *DockerLogWatch) parseJSON(body []byte) (map[string]interface{}, error) {
	var jsonResult map[string]interface{}
	err := json.Unmarshal(body, &jsonResult)
	if err != nil {
		return nil, err
	}

	return jsonResult, nil
}

func (w *DockerLogWatch) emitEvents(body map[string]interface{}) {
	// search for & emit events
	for _, ev := range w.Events {
		newev, err := ev.New(body, timesync.Now())
		if err != nil {
			zap.S().Warnf("event construction error %v", err)

			continue
		}

		if newev == nil {
			// nothing to do
			continue
		}

		if err := emit.Ev(w, newev); err != nil {
			zap.S().Error(err)

			continue
		}

		// stop if at least one event was emitted
		// for the same buffer
		break
	}
}

// StartUnsafe starts the goroutine for discovering and tailing a
// container's logs.
func (w *DockerLogWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	if len(w.Regex) < 1 {
		w.Log.Error("missing required container 'regex', nothing to watch")

		return
	}

	var (
		rc     io.ReadCloser
		ctx    context.Context
		cancel context.CancelFunc
		err    error
	)

	newEventStream := func() bool {
		// Retry forever to establish the stream. Ensures periodic retries
		// according to the specified interval and probes the stop channel
		// for exit point.
		for {
			select {
			case <-w.StopKey:
				return true
			default:
			}
			time.Sleep(w.RetryIntv)

			// retry forever to re-establish the stream.
			ctx, cancel = context.WithCancel(context.Background())

			rc, err = w.repairLogStream(ctx)
			if err != nil {
				w.Log.Warnw("error getting stream", zap.Error(err))
				continue
			}

			return false
		}
	}

	if stopped := newEventStream(); stopped {
		return
	}

	lastErr := errors.New("node log missing")
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		hdr := make([]byte, 8)
		buf := make([]byte, 1024)

		for {
			select {
			case <-w.StopKey:
				rc.Close()

				cancel()
				return
			default:
			}

			n, err := rc.Read(hdr)
			if err != nil {
				lastErr = err
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					w.Log.Errorw("error reading header", zap.Error(err), "reader", rc)

					continue
				}

				w.Log.Error("EOF error while reading header, will try to recover in 5s")
				time.Sleep(5 * time.Second)

				ctx := map[string]interface{}{
					model.NodeIDKey:      global.BlockchainNode.NodeID(),
					model.NodeTypeKey:    global.BlockchainNode.NodeRole(),
					model.NodeVersionKey: global.BlockchainNode.NodeVersion(),
				}

				ev, err := model.NewWithCtx(ctx, model.AgentNodeLogMissingName, timesync.Now())
				if err != nil {
					w.Log.Errorw("error creating event: ", zap.Error(err))
				} else {
					if err := emit.Ev(w, ev); err != nil {
						w.Log.Errorw("error emitting event: ", zap.Error(err))
					}
				}

				cancel()
				if err := rc.Close(); err != nil {
					w.Log.Errorw("error closing docker logs stream", zap.Error(err))
				}

				if stopped := newEventStream(); stopped {
					return
				}

				continue
			}

			if n < len(hdr) {
				w.Log.Error("read unexpected number of header bytes")

				continue
			}

			count := binary.BigEndian.Uint32(hdr[4:])
			if int(count) > cap(buf) {
				if count < maxLineBytes {
					w.Log.Debugf("increasing log buffer capacity to %d bytes (from %d)", count, cap(buf))

					buf = make([]byte, count)
				}
			} else {
				buf = buf[:count]
			}

			n, err = rc.Read(buf)
			if err != nil {
				lastErr = err
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					w.Log.Errorw("error reading data", zap.Error(err), "reader", rc)

					continue
				}

				w.Log.Error("EOF error while reading log data, will try to recover log streaming immediately")
				ctx := map[string]interface{}{
					model.NodeIDKey:      global.BlockchainNode.NodeID(),
					model.NodeTypeKey:    global.BlockchainNode.NodeRole(),
					model.NodeVersionKey: global.BlockchainNode.NodeVersion(),
				}

				ev, err := model.NewWithCtx(ctx, model.AgentNodeLogMissingName, timesync.Now())
				if err != nil {
					w.Log.Errorw("error creating event: ", zap.Error(err))
				} else {
					if err := emit.Ev(w, ev); err != nil {
						w.Log.Errorw("error emitting event: ", zap.Error(err))
					}
				}

				cancel()
				if err := rc.Close(); err != nil {
					w.Log.Errorw("error closing docker logs stream: ", zap.Error(err))
				}

				if stopped := newEventStream(); stopped {
					return
				}

				continue
			}

			if n < len(buf) {
				w.Log.Error("read unexpected number of data bytes")

				continue
			}

			if lastErr != nil {
				ctx := map[string]interface{}{
					model.NodeIDKey:      global.BlockchainNode.NodeID(),
					model.NodeTypeKey:    global.BlockchainNode.NodeRole(),
					model.NodeVersionKey: global.BlockchainNode.NodeVersion(),
				}
				ev, err := model.NewWithCtx(ctx, model.AgentNodeLogFoundName, timesync.Now())

				if err != nil {
					w.Log.Errorw("error creating event: ", zap.Error(err))
				} else {
					if err := emit.Ev(w, ev); err != nil {
						w.Log.Errorw("error emitting event: ", zap.Error(err))
					}
				}
				lastErr = nil
			}

			jsonMap, err := w.parseJSON(buf)
			if err != nil {
				w.Log.Errorw("error parsing events from log line:", zap.Error(err))

				continue
			}

			w.emitEvents(jsonMap)
		}
	}()
}

// Stop stops the watch.
func (w *DockerLogWatch) Stop() {
	w.Watch.Stop()
}
