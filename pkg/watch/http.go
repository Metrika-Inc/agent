package watch

// *** HttpGetWatch ***

type HttpGetWatchConf struct {
	Url string
}

type HttpGetWatch struct {
	HttpGetWatchConf
	Watch

	Trigger   Watcher
	triggerCh chan interface{}
}

func NewHttpGetWatch(conf HttpGetWatchConf, trigger Watcher) *HttpGetWatch {
	w := new(HttpGetWatch)

	return w
}
