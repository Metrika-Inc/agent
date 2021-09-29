package watch

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"time"
)

type FileWatchConf struct {
	Path string
}

// *** FileReadWatch ***

type FileReadWatchConf struct {
	FileWatchConf

	/// Interval is used when creating a default timer. If a timer is provided in the constructor, this value is ignored.
	Interval time.Duration
}

type FileReadWatch struct {
	FileReadWatchConf
	Watch

	TimerWatch *TimerWatch
	timerCh    chan interface{}
}

func NewFileReadWatch(conf FileReadWatchConf, timerWatch *TimerWatch) *FileReadWatch {
	w := new(FileReadWatch)
	w.Watch = NewWatch()
	w.FileReadWatchConf = conf
	w.TimerWatch = timerWatch
	w.timerCh = make(chan interface{}, 1)
	return w
}

func (w *FileReadWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	// Check for missing path
	if w.Path == "" {
		panic("Missing required argument 'path'") //todo handle
	}

	// Create default timer if not provided
	if w.TimerWatch == nil {
		w.TimerWatch = NewTimerWatch(TimerWatchConf{
			Interval: w.Interval,
		})
	}

	// Subscribe/Start timer
	w.TimerWatch.Subscribe(w.timerCh)
	Start(w.TimerWatch)

	// Listen to events
	go w.handleTimer()
}

func (w *FileReadWatch) handleTimer() {
	for {
		select {
		case <-w.timerCh:
			// Read file
			file, err := ioutil.ReadFile(w.Path)
			if err != nil {
				log.Errorf("[FileReadWatch] Failed to read file %s: %v\n", w.Path, err)
				continue
			}

			// Emit message
			w.Emit(file)

		case <-w.StopKey:
			return

		}
	}
}

// *** FileNotifyWatch ***

type FileNotifyWatchConf struct {
	FileWatchConf

	Ops fsnotify.Op
}

// FileNotifyWatch / FileNotifyWatch watches a file using fsnotify...
///
type FileNotifyWatch struct {
	FileNotifyWatchConf
	Watch

	watcher *fsnotify.Watcher
}

func NewFileNotifyWatch(conf FileNotifyWatchConf) *FileNotifyWatch {
	w := new(FileNotifyWatch)
	w.Watch = NewWatch()
	w.FileNotifyWatchConf = conf
	return w
}

func (w *FileNotifyWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	// Check for missing path
	if w.Path == "" {
		panic("Missing required argument 'path'") //todo handle
	}

	go func() {
		w.tryCreateWatcher()
		w.pollEvents()
	}()
}

func (w *FileNotifyWatch) Stop() {
	w.Watch.Stop()

	err := w.watcher.Close()
	if err != nil {
		log.Errorf("[FileNotifyWatch] Failed to stop watch service: %v\n", err)
	}
}

func (w *FileNotifyWatch) pollEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok && w.Running {
				w.tryCreateWatcher() // Attempt to restart service
			}

			if event.Op&w.Ops != 0 {
				// Event is in the given options (we do not know which), emit event
				log.Tracef("[FileNotifyWatch] Emitted '%s' for '%s'\n", event.Op.String(), w.Path)
				w.Emit(event.Op)
			} // Ignore other events

		case err, _ := <-w.watcher.Errors:
			if !w.Running {
				return
			}

			// Log on error, we will try to restart if channel closed anyway
			if err != nil {
				log.Errorf("[FileNotifyWatch] Error from watch service (EVENTS WERE LOST): %v\n", err)
			}

			w.tryCreateWatcher() // Attempt to restart service

		case <-w.StopKey:
			return
		}
	}
}

func (w *FileNotifyWatch) createWatcher() error {
	// Try to close, but we don't care if it fails
	if w.watcher != nil {
		_ = w.watcher.Close()
	}

	// Create new
	var err error
	w.watcher, err = fsnotify.NewWatcher()
	return err
}

func (w *FileNotifyWatch) tryCreateWatcher() {
	for {
		err := w.createWatcher()
		if err != nil {
			log.Errorf("[FileNotifyWatch] Failed to create watch service: %v\n", err)
			continue
		}

		err = w.watcher.Add(w.Path)
		if err != nil {
			log.Errorf("[FileNotifyWatch] Failed to watch file '%s': %v\n", w.Path, err)
			continue
		}

		break
	}
}
