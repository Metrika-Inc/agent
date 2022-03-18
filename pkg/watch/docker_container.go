package watch

import (
	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/pkg/timesync"
	"context"
	"fmt"
	"sync"
	"time"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

type ContainerWatchConf struct {
	Regex []string
    RetryIntv time.Duration
}

type ContainerWatch struct {
	ContainerWatchConf
	Watch

	watchCh      chan interface{}
	stopListCh   chan interface{}
	stopEventsch chan interface{}
	mutex        *sync.RWMutex
	container    dt.Container
}

func NewContainerWatch(conf ContainerWatchConf) *ContainerWatch {
	w := new(ContainerWatch)
	w.Watch = NewWatch()
	w.ContainerWatchConf = conf
	w.watchCh = make(chan interface{}, 1)
	w.stopListCh = make(chan interface{}, 1)
	w.stopEventsch = make(chan interface{}, 1)
	w.mutex = new(sync.RWMutex)

    if w.RetryIntv == 0 {
        w.RetryIntv = defaultRetryIntv
    }

	return w
}

func (w *ContainerWatch) updateContainer(container dt.Container) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.container = container
}

func (w *ContainerWatch) repairEventStream(ctx context.Context) (
	<-chan events.Message, <-chan error, error) {

	containers, err := utils.GetRunningContainers()
	if err != nil {
		return nil, nil, err
	}

	container, err := utils.MatchContainer(containers, w.Regex)
	if err != nil {
		ev, err := model.NewWithCtx(
			map[string]interface{}{"error": err.Error()},
			model.AgentNodeDownName,
			model.AgentNodeDownDesc)
		if err != nil {
			return nil, nil, err
		}

		if err := emit.Ev(w, timesync.Now(), ev); err != nil {
			return nil, nil, fmt.Errorf("error emitting event about error: %v", err)
		}

		return nil, nil, err
	}
	w.updateContainer(container)

	ev, err := model.NewWithCtx(map[string]interface{}{
		"image":  container.Image,
		"state":  container.State,
		"status": container.Status,
		"names":  container.Names[0],
	}, model.AgentNodeUpName, model.AgentNodeUpDesc)
	if err != nil {
		return nil, nil, err
	}

	if err := emit.Ev(w, timesync.Now(), ev); err != nil {
		return nil, nil, fmt.Errorf("error emitting event about error: %v", err)
	}

	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("status", "start")
	filter.Add("status", "stop")
	filter.Add("status", "kill")
	filter.Add("status", "die")
	filter.Add("container", w.container.ID)
	options := dt.EventsOptions{Filters: filter}

	msgchan, errchan, err := utils.DockerEvents(ctx, options)
	if err != nil {
		return nil, nil, err
	}

	return msgchan, errchan, nil
}

func (w *ContainerWatch) parseDockerEvent(m events.Message) (*model.Event, error) {
	var ev *model.Event
	var err error
	ctx := map[string]interface{}{"id": m.ID, "status": m.Status, "action": m.Action}

	switch m.Status {
	case "start":
		ev, err = model.NewWithCtx(ctx, model.AgentNodeUpName, model.AgentNodeUpDesc)
	case "restart":
		ev, err = model.NewWithCtx(ctx, model.AgentNodeRestartName, model.AgentNodeRestartDesc)
	case "die":
		exitCode, ok := m.Actor.Attributes["exitCode"]
		if ok {
			ctx["exitCode"] = exitCode
		}

		ev, err = model.NewWithCtx(ctx, model.AgentNodeDownName, model.AgentNodeDownDesc)
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
        sleep   bool
	)

	newEventStream := func() {
		for {
			if sleep {
				time.Sleep(w.RetryIntv)
			} else {
				sleep = true
			}

			// retry forever to re-establish the stream.
			ctx, cancel = context.WithCancel(context.Background())

			if msgchan, errchan, err = w.repairEventStream(ctx); err != nil {
				w.Log.Error("error getting docker event stream:", err)

				continue
			}

			w.Log.Info("docker event stream ready")
			break
		}
	}
	newEventStream()

	w.Wg.Add(1)
	go func() {
		defer w.Wg.Done()

		for {
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

				if err := emit.Ev(w, timesync.Now(), ev); err != nil {
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
