package watch

import (
	"context"
	"fmt"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/discover"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/internal/pkg/global"
	"agent/pkg/timesync"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"
)

type ContainerWatchConf struct {
	Regex     []string
	RetryIntv time.Duration
}

type ContainerWatch struct {
	ContainerWatchConf
	Watch

	watchCh         chan interface{}
	stopListCh      chan interface{}
	stopEventsch    chan interface{}
	containerGone   bool
	lastContainerID string
}

func NewContainerWatch(conf ContainerWatchConf) *ContainerWatch {
	w := new(ContainerWatch)
	w.Watch = NewWatch()
	w.ContainerWatchConf = conf
	w.watchCh = make(chan interface{}, 1)
	w.stopListCh = make(chan interface{}, 1)
	w.stopEventsch = make(chan interface{}, 1)

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
		ev, errev := model.NewWithCtx(
			map[string]interface{}{"old_container_id": w.lastContainerID},
			model.AgentNodeDownName, timesync.Now())
		if errev != nil {
			return nil, nil, fmt.Errorf("errors: %v; %v", err, errev)
		}

		if err := emit.Ev(w, ev); err != nil {
			return nil, nil, fmt.Errorf("error emitting event about error: %v", err)
		}

		return nil, nil, err
	}
	ev, err := model.NewWithCtx(map[string]interface{}{
		"image":        container.Image,
		"state":        container.State,
		"status":       container.Status,
		"node_id":      discover.NodeID(),
		"node_type":    discover.NodeType(),
		"node_version": discover.NodeVersion(),
	}, model.AgentNodeUpName, timesync.Now())
	if err != nil {
		return nil, nil, err
	}

	if err := emit.Ev(w, ev); err != nil {
		return nil, nil, fmt.Errorf("error emitting event about error: %v", err)
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
		return nil, nil, err
	}

	w.lastContainerID = container.ID

	return msgchan, errchan, nil
}

func (w *ContainerWatch) parseDockerEvent(m events.Message) (*model.Event, error) {
	var ev *model.Event
	var err error
	ctx := map[string]interface{}{
		"status":       m.Status,
		"action":       m.Action,
		"container_id": m.Actor.ID,
		"node_id":      discover.NodeID(),
		"node_type":    discover.NodeType(),
		"node_version": discover.NodeVersion(),
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

func (w *ContainerWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	if len(w.Regex) < 1 {
		w.Log.Error("missing required argument 'regex', nothing to watch")

		return
	}

	var (
		msgchan <-chan events.Message
		errchan <-chan error
		ctx     context.Context
		cancel  context.CancelFunc
		err     error
	)

	var sleepd time.Duration
	newEventStream := func() {
		// Retry forever to re-establish the stream. Ensures
		// periodic retries according to the specified interval and
		// probes the stop channel for exit point.
		for {
			if sleepd != 0 {
				time.Sleep(sleepd)
			} else {
				sleepd = defaultRetryIntv
			}

			select {
			case <-w.StopKey:
				return
			default:
			}

			ctx, cancel = context.WithCancel(context.Background())
			if msgchan, errchan, err = w.repairEventStream(ctx); err != nil {
				w.Log.Warnw("getting docker event stream failed", zap.Error(err))
				w.containerGone = true

				continue
			}

			w.containerGone = false
			w.Log.Info("docker event stream ready")

			return
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
				} else {
					if ev == nil {
						// nothing to do
						continue
					}
				}

				if err := emit.Ev(w, ev); err != nil {
					w.Log.Error("error emitting docker event", err)
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
