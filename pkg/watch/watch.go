package watch

import (
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	PrometheusWatchPrefix           = "prometheus"
	AlgorandWatchPrefix             = "algorand"
	AlgorandNodeRestart   WatchType = "algorand.node.restart"
	PEFWatchPrefix                  = "pef"
)

type WatchType string

func (w WatchType) IsPrometheus() bool {
	return strings.HasPrefix(string(w), PrometheusWatchPrefix)
}

func (w WatchType) IsAlgorand() bool {
	return strings.HasPrefix(string(w), AlgorandWatchPrefix)
}

func (w WatchType) IsPEF() bool {
	return strings.HasPrefix(string(w), PEFWatchPrefix)
}

type Watcher interface {
	StartUnsafe()
	Stop()

	Subscribe(chan<- interface{})

	once() *sync.Once
}

func Start(watcher Watcher) {
	watcher.once().Do(watcher.StartUnsafe)
}

type Watch struct {
	Running bool

	StopKey chan bool

	startOnce *sync.Once
	listeners []chan<- interface{}
	Log       *zap.SugaredLogger
}

func NewWatch() Watch {
	return Watch{
		Running:   false,
		StopKey:   make(chan bool, 1),
		startOnce: &sync.Once{},
		Log:       zap.S(),
	}
}

func (w *Watch) StartUnsafe() {
	w.Running = true
}

func (w *Watch) Stop() {
	if !w.Running {
		return
	}
	w.Running = false

	w.StopKey <- true
}

func (w *Watch) once() *sync.Once {
	return w.startOnce
}

// Subscription mechanism

func (w *Watch) Subscribe(handler chan<- interface{}) {
	w.listeners = append(w.listeners, handler)
}

func (w *Watch) Emit(message interface{}) {
	for _, handler := range w.listeners {
		handler <- message
	}
}
