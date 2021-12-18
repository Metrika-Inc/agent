package watch

import (
	"sync"
)

var (
	PrometheusNetNetstat WatchType = "prometheus.proc.net.netstat_linux"
	PrometheusNetARP     WatchType = "prometheus.proc.net.arp_linux"
	PrometheusStat       WatchType = "prometheus.proc.stat_linux"
	PrometheusConntrack  WatchType = "prometheus.proc.conntrack_linux"
	PrometheusCPU        WatchType = "prometheus.proc.cpu"
	PrometheusDiskStats  WatchType = "prometheus.proc.diskstats"
	PrometheusEntropy    WatchType = "prometheus.proc.entropy"
	PrometheusFileFD     WatchType = "prometheus.proc.filefd"
	PrometheusFilesystem WatchType = "prometheus.proc.filesystem"
	PrometheusLoadAvg    WatchType = "prometheus.proc.loadavg"
	PrometheusMemInfo    WatchType = "prometheus.proc.meminfo"
	PrometheusNetClass   WatchType = "prometheus.proc.netclass"
	PrometheusNetDev     WatchType = "prometheus.proc.netdev"
	PrometheusSockStat   WatchType = "prometheus.proc.sockstat"
	PrometheusTextfile   WatchType = "prometheus.proc.textfile"
	PrometheusTime       WatchType = "prometheus.time"
	PrometheusUname      WatchType = "prometheus.uname"
	PrometheusVMStat     WatchType = "prometheus.vmstat"
	AlgorandNodeRestart  WatchType = "algorand.node.restart"
)

type WatchType string

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
}

func NewWatch() Watch {
	return Watch{
		Running:   false,
		StopKey:   make(chan bool, 1),
		startOnce: &sync.Once{},
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
