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
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

const maxLineBytes = uint32(1024 * 1024)

type DockerLogWatchConf struct {
	Regex  []string
	Events map[string]model.FromContext
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

		if err := emit.EmitEvent(w, timesync.Now(), newev); err != nil {
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
		err    error
	)

	wg := &sync.WaitGroup{}
	newStream := func() {
		defer wg.Done()

		for {
			// retry forever to re-establish the stream.
			ctx, cancel = context.WithCancel(context.Background())

			rc, err = w.repairLogStream(ctx)
			if err != nil {
				w.Log.Error("error getting stream:", err)
				time.Sleep(5 * time.Second)

				continue
			}

			ev := model.New(model.AgentNodeLogFoundName, model.AgentNodeLogFoundDesc)
			emit.EmitEvent(w, timesync.Now(), ev)

			break
		}
	}

	wg.Add(1)
	go newStream()
	wg.Wait()

	go func() {

		hdr := make([]byte, 8)
		buf := make([]byte, 1024)

		for {
			n, err := rc.Read(hdr)
			if err != nil {
				if err == io.EOF {
					w.Log.Error("EOF reached (hdr), resetting stream")

					ev := model.New(model.AgentNodeLogMissingName, model.AgentNodeLogMissingDesc)
					emit.EmitEvent(w, timesync.Now(), ev)

					cancel()

					wg.Add(1)
					go newStream()
					wg.Wait()
				} else {
					w.Log.Error("error reading header", err)
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
					w.Log.Warnf("increasing log buffer capacity to %d bytes (from %d)", count, cap(buf))

					buf = make([]byte, count)
				}
			} else {
				buf = buf[:count]
			}

			n, err = rc.Read(buf)
			if err != nil {
				if err == io.EOF {
					w.Log.Error("EOF reached (data), resetting stream")
					cancel()

					wg.Add(1)
					go newStream()
					wg.Wait()
				} else {
					w.Log.Error("error reading data", err)

				}

				continue
			}

			if n < len(buf) {
				w.Log.Error("read unexpected number of data bytes")

				continue
			}

			jsonMap, err := w.parseJSON(buf)
			if err != nil {
				w.Log.Error("error parsing events from log line:", err)
			}

			w.emitEvents(jsonMap)
		}
	}()
}

func (w *DockerLogWatch) Stop() {
	w.Watch.Stop()
}
