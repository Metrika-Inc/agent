package watch

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

type FileWatchConf struct {
	Path string
}

// *** FileReadWatch ***

type FileReadWatchConf FileWatchConf

type FileReadWatch struct {
	FileWatchConf
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

	go func() {
		w.tryCreateWatcher()
		w.PollEvents()
	}()
}

func (w *FileNotifyWatch) Stop() {
	w.Watch.Stop()

	err := w.watcher.Close()
	if err != nil {
		log.Errorf("[FileNotifyWatch] Failed to stop watch service: %v\n", err)
	}
}

func (w *FileNotifyWatch) PollEvents() {
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
