package watch

import (
	"agent/api/v1/model"
	"agent/internal/pkg/discover/utils"
	"agent/internal/pkg/emit"
	"agent/pkg/timesync"
	"context"
	"errors"
	"sync"
	"time"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

type ContainerWatchConf struct {
	Regex []string
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
	} else {
		container, err := utils.MatchContainer(containers, w.Regex)
		if err != nil {
			emit.EmitEvent(w, timesync.Now(), model.NewWithCtx(
				map[string]interface{}{"error": err.Error()},
				model.AgentNodeDownName,
				model.AgentNodeDownDesc))

			return nil, nil, err
		}
		w.updateContainer(container)

		emit.EmitEvent(w, timesync.Now(), model.NewWithCtx(
			map[string]interface{}{
				"image":  container.Image,
				"state":  container.State,
				"status": container.Status,
				"names":  container.Names,
			},
			model.AgentNodeUpName, model.AgentNodeUpDesc))
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

	switch m.Status {
	case "start":
		ev = model.NewWithCtx(
			map[string]interface{}{
				"image":  w.container.Image,
				"state":  w.container.State,
				"status": w.container.Status,
				"names":  w.container.Names,
			},
			model.AgentNodeUpName, model.AgentNodeUpDesc)
	case "restart":
		ev = model.NewWithCtx(
			map[string]interface{}{"id": m.ID},
			model.AgentNodeRestartName, model.AgentNodeRestartDesc)
	case "die":
		signal, ok := m.Actor.Attributes["signal"]
		if !ok {
			return nil, errors.New("'signal' attribute missing from docker kill event")
		}

		ev = model.NewWithCtx(
			map[string]interface{}{"id": m.ID, "signal": signal},
			model.AgentNodeDownName, model.AgentNodeDownDesc,
		)
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

	wg := &sync.WaitGroup{}
	newEventStream := func() {
		defer wg.Done()

		for {
			// retry forever to re-establish the stream.
			ctx, cancel = context.WithCancel(context.Background())
			if msgchan, errchan, err = w.repairEventStream(ctx); err != nil {
				w.Log.Error("error getting docker event stream:", err)
				time.Sleep(5 * time.Second)

				continue
			}

			w.Log.Info("docker event stream ready")
			break
		}
	}
	wg.Add(1)
	go newEventStream()

	// wait for event stream channels to be ready
	// before entering their select{}.
	wg.Wait()

	go func() {
		for {
			select {
			case m := <-msgchan:
				w.Log.Debugf("docker event message: ID:%s, status:%s, signal:%s",
					m.ID[:8], m.Status, m.Actor.Attributes["signal"])

				ev, err := w.parseDockerEvent(m)
				if err != nil {
					w.Log.Error("error emitting docker event", err)
				}

				emit.EmitEvent(w, timesync.Now(), ev)
			case err := <-errchan:
				w.Log.Debugf("docker event error: %v, will try to recover the stream", err)
				cancel()

				wg.Add(1)
				newEventStream()
			case <-w.StopKey:
				cancel()

				return
			}
		}
	}()
}
