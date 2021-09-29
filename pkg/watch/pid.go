package watch

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strconv"
	"strings"
)

type DotPidWatchConf struct {
	Path string
}

type DotPidWatch struct {
	DotPidWatchConf
	Watch

	FileWatch   Watcher
	fileWatchCh chan interface{}
}

func NewDotPidWatch(conf DotPidWatchConf) *DotPidWatch {
	w := new(DotPidWatch)
	w.Watch = NewWatch()
	w.DotPidWatchConf = conf
	w.FileWatch = NewFileNotifyWatch(FileNotifyWatchConf{
		FileWatchConf: FileWatchConf{
			Path: conf.Path,
		},
		AllowNotExists: true,
		Ops:            fsnotify.Remove | fsnotify.Create,
	})
	w.fileWatchCh = make(chan interface{}, 1)
	return w
}

func (w *DotPidWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	// Check for missing path
	if w.Path == "" {
		panic("Missing required argument 'path'") //todo handle
	}

	// Subscribe & start file watch
	w.FileWatch.Subscribe(w.fileWatchCh)
	Start(w.FileWatch)

	go w.handlePidFileChanges()

	// Read initial pid
	initial, err := w.readPid()
	if err != nil {
		w.Emit(0)
	} else {
		w.Emit(initial)
	}
}

func (w *DotPidWatch) handlePidFileChanges() {
	for {
		select {
		case op := <-w.fileWatchCh:
			// Set new pid when changed
			if op.(fsnotify.Op)&fsnotify.Create == fsnotify.Create {
				pid, err := w.readPid()
				if err != nil {
					log.Errorf("[DotPidWatch] Failed to read pid file on create: %v\n", err)
					continue
				}

				// Emit new pid
				w.Emit(pid)

			} else {
				// op == REMOVE
				// Emit -1
				w.Emit(0)
			}

		case <-w.StopKey:
			return
		}
	}
}

func (w *DotPidWatch) readPid() (int, error) {
	bytes, err := ioutil.ReadFile(w.Path)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.Trim(string(bytes), "\n"))
}
