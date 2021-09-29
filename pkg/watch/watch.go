package watch

type WatchService interface {
	Start()
	Stop()
}

type Watch struct {
	Running bool

	StartFn func()
	StopFn  func()
	StopKey chan bool

	listeners []func(interface{})
}

func NewWatch() Watch {
	return Watch{
		Running: false,
		StartFn: func() {},
		StopFn:  func() {},
		StopKey: make(chan bool, 1),
	}
}

func (w *Watch) Start() {
	if w.Running {
		return
	}
	w.Running = true

	w.StartFn()
}

func (w *Watch) Stop() {
	if !w.Running {
		return
	}
	w.Running = false

	w.StopFn()
	w.StopKey <- true
}

// Subscription mechanism

func (w *Watch) Subscribe(handler func(interface{})) {
	w.listeners = append(w.listeners, handler)
}

func (w *Watch) Emit(message interface{}) {
	for _, handler := range w.listeners {
		handler(message)
	}
}
