package watch

import (
	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/pkg/timesync"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const maxLineBytes = uint32(1024 * 1024)

var defaultRetryIntv = 3 * time.Second

type DockerLogWatchConf struct {
	Regex     []string
	Events    map[string]model.FromContext
	RetryIntv time.Duration
}

type DockerLogWatch struct {
	DockerLogWatchConf
	Watch

	rc io.ReadCloser
}

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
		// Timestamps: true,
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "0",
		// TODO: restore offset from a WAL.
		//Since: "2022-02-25T19:14:59.721119832Z",
	}

	rc, err := utils.DockerLogs(ctx, w.Regex[0], options)
	if err != nil {
		return nil, err
	}

	return rc, nil
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
		newev, err := ev.New(body)
		if err != nil {
			zap.S().Warnf("event construction error %v", err)

			continue
		}

		if newev == nil {
			// nothing to do
			continue
		}

		if err := emit.Ev(w, timesync.Now(), newev); err != nil {
			zap.S().Error(err)

			continue
		}

		// stop if at least one event was emitted
		// for the same buffer
		break
	}
}

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
		sleep  bool
		err    error
	)

	newStream := func() {
		for {
			if sleep {
				time.Sleep(w.RetryIntv)
			} else {
				sleep = true
			}

			// retry forever to re-establish the stream.
			ctx, cancel = context.WithCancel(context.Background())

			rc, err = w.repairLogStream(ctx)
			if err != nil {
				w.Log.Error("error getting stream:", err)

				continue
			}

			ev, err := model.New(model.AgentNodeLogFoundName, model.AgentNodeLogFoundDesc)
			if err != nil {
				w.Log.Error("error creating event: ", err)
			} else {
				if err := emit.Ev(w, timesync.Now(), ev); err != nil {
					w.Log.Error("error emitting event: ", err)
				}
			}

			break
		}
	}
	newStream()

	w.Wg.Add(1)
	go func() {
		defer w.Wg.Done()

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
				if err == io.EOF {
					w.Log.Error("EOF reached (hdr), resetting stream")

					ev, err := model.New(model.AgentNodeLogMissingName, model.AgentNodeLogMissingDesc)
					if err != nil {
						w.Log.Error("error creating event: ", err)
					} else {
						if err := emit.Ev(w, timesync.Now(), ev); err != nil {
							w.Log.Error("error emitting event: ", err)
						}
					}

					cancel()
					rc.Close()
					newStream()
				} else {
					w.Log.Errorw("error reading header", zap.Error(err))
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
				if err == io.EOF {
					w.Log.Error("EOF reached (data), resetting stream")

					ev, err := model.New(model.AgentNodeLogMissingName, model.AgentNodeLogMissingDesc)
					if err != nil {
						w.Log.Error("error creating event: ", err)
					} else {
						if err := emit.Ev(w, timesync.Now(), ev); err != nil {
							w.Log.Error("error emitting event: ", err)
						}
					}

					cancel()
					rc.Close()
					newStream()
				} else {
					w.Log.Errorw("error reading data", zap.Error(err))
				}

				continue
			}

			if n < len(buf) {
				w.Log.Error("read unexpected number of data bytes")

				continue
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

func (w *DockerLogWatch) Stop() {
	w.Watch.Stop()
}
