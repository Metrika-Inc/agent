package watch

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
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

	w.Wg.Add(1)
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
	defer w.Wg.Done()

	for {
		select {
		case op := <-w.fileWatchCh:
			// Set new pid when changed
			if op.(fsnotify.Op)&fsnotify.Create == fsnotify.Create {
				pid, err := w.readPid()
				if err != nil {
					w.Log.Errorw("Failed to read pid file on create", zap.Error(err))
					continue
				}

				// Emit new pid
				w.Emit(pid)
				w.Log.Debugw("New pid emitted", "pid", pid)

			} else {
				// op == REMOVE
				w.Emit(0)
				w.Log.Debug("0 pid emitted")
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
