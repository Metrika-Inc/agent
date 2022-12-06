package watch

import (
	"reflect"
	"sync"

	"go.uber.org/zap"
)

// DefaultWatchRegistry is the default watcher registry used by the agent.
var DefaultWatchRegistry WatchersRegisterer

func init() {
	defaultWatchRegistrar := &Registry{
		watch:      []*WatcherInstance{},
		Mutex:      &sync.Mutex{},
		watcherMap: make(map[Watcher]struct{}),
	}

	DefaultWatchRegistry = defaultWatchRegistrar
}

// WatchersRegisterer is an interface for enabling agent watchers.
type WatchersRegisterer interface {
	Register(w ...Watcher) error
	Start(ch ...chan<- interface{}) error
	RegisterAndStart(w Watcher, ch ...chan<- interface{}) error
	Stop()
	Wait()
}

// Registry is an implementation of WatchersRegisterer.
// It controls the agent watchers' life cycle.
type Registry struct {
	watch []*WatcherInstance

	// watcherMap is used to track registered watchers
	// and ensure idempotency when calling register
	watcherMap map[Watcher]struct{}
	*sync.Mutex
}

// WatcherInstance describes a state of a single
// watcher that's inside the registry.
type WatcherInstance struct {
	started bool
	watcher Watcher
	*sync.Mutex
}

// Register registrers one or more watchers.
// Register is idempotent - trying to register
// an already registered watcher will be a no-op.
func (r *Registry) Register(w ...Watcher) error {
	r.Lock()
	defer r.Unlock()
	for _, watcher := range w {
		_, err := r.register(watcher)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Registry) register(watcher Watcher) (*WatcherInstance, error) {
	if _, ok := r.watcherMap[watcher]; ok {
		zap.S().Warnw("duplicate register attempt watcher, skipping",
			"type", reflect.TypeOf(watcher).String())
		return nil, nil
	}
	instance := &WatcherInstance{
		watcher: watcher,
		Mutex:   &sync.Mutex{},
	}
	r.watch = append(r.watch, instance)
	r.watcherMap[watcher] = struct{}{}
	return instance, nil
}

// RegisterAndStart attempts to register and start a single watcher.
func (r *Registry) RegisterAndStart(w Watcher, ch ...chan<- interface{}) error {
	r.Lock()
	defer r.Unlock()

	instance, err := r.register(w)
	if err != nil {
		return err
	}

	return start(ch, instance)
}

// Start starts a watch by subscribing to one or more channels
// for emitting collected data.
// Calling Start multiple times will start watchers that haven't
// been started, and will act as a no-op for already running watchers, even
// if ch parameter is different.
func (r *Registry) Start(ch ...chan<- interface{}) error {
	r.Lock()
	defer r.Unlock()
	return start(ch, r.watch...)
}

func start(ch []chan<- interface{}, instances ...*WatcherInstance) error {
	for _, w := range instances {
		if w.started {
			continue
		}

		for _, c := range ch {
			w.watcher.Subscribe(c)
		}

		go func(w Watcher) {
			Start(w)
		}(w.watcher)
		w.started = true
	}

	return nil
}

// Stop stops all registered watches
func (r *Registry) Stop() {
	r.Lock()
	defer r.Unlock()
	for _, w := range r.watch {
		w.watcher.Stop()
		w.started = false
	}
}

// Wait for all registered watches to finish
func (r *Registry) Wait() {
	for _, w := range r.watch {
		w.watcher.Wait()
	}
}
