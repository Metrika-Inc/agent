package watch

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
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
		// Emit once on start for initial value
		w.emitFile()

		select {
		case <-w.timerCh:
			// Emit on time interval
			w.emitFile()

		case <-w.StopKey:
			return

		}
	}
}

func (w *FileReadWatch) emitFile() {
	// Read file
	file, err := ioutil.ReadFile(w.Path)
	if err != nil {
		log.Errorf("[FileReadWatch] Failed to read file %s: %v\n", w.Path, err)
		return
	}

	// Emit message
	w.Emit(file)
}

// *** FileNotifyWatch ***

type FileNotifyWatchConf struct {
	FileWatchConf

	AllowNotExists bool
	Ops            fsnotify.Op
}

// FileNotifyWatch / FileNotifyWatch watches a file using fsnotify...
///
type FileNotifyWatch struct {
	FileNotifyWatchConf
	Watch

	watcher *fsnotify.Watcher
}

func NewFileNotifyWatch(conf FileNotifyWatchConf) Watcher {
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
		if !w.Running {
			return
		}

		select {
		case event, ok := <-w.watcher.Events:
			if !ok && w.Running {
				w.tryCreateWatcher() // Attempt to restart service
			}

			if event.Op&w.Ops != 0 {
				// Event is in the given options (we do not know which), emit event
				log.Tracef("[FileNotifyWatch] Emitted '%s' for '%s'\n", event.Op.String(), w.Path)
				w.Emit(event.Op)
			}

			// We cannot watch files which don't exist, so we must wait to resubscribe after the file is back
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				w.waitForFileCreation()
			}

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
	tryCreate := func() bool {
		err := w.createWatcher()
		if err != nil {
			log.Errorf("[FileNotifyWatch] Failed to create watch service: %v\n", err)
			return false
		}

		err = w.watcher.Add(w.Path)
		if err != nil {
			if os.IsNotExist(err) && w.AllowNotExists {
				// This is an acceptable error, we just need to wait for creation
				w.waitForFileCreation()
				return true
			}

			log.Errorf("[FileNotifyWatch] Failed to watch file '%s': %v\n", w.Path, err)
			return false
		}

		return true
	}

	// Try once immediately
	if tryCreate() {
		return
	}

	for {
		select {
		case <-time.After(time.Second):
			if tryCreate() {
				return
			}

		case <-w.StopKey:
			return
		}
	}
}

func (w *FileNotifyWatch) waitForFileCreation() {
	for {
		select {
		case <-time.After(10 * time.Millisecond):
			if _, err := os.Stat(w.Path); err == nil {
				// File exists, subscribe to it and emit fake event
				err := w.watcher.Add(w.Path)
				if err != nil {
					w.tryCreateWatcher() // Watcher is gone, try again
				}

				// Simulate create event
				w.Emit(fsnotify.Create)
				log.Tracef("[FileNotifyWatch] Emitted '%s' for '%s'\n", fsnotify.Create.String(), w.Path)

				return
			}

			// Continue waiting

		case <-w.StopKey:
			return
		}
	}
}
