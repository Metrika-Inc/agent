package watch

import (
	. "agent/pkg/watch"
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"os"
)

// *** AlgorandLogWatch ***

type AlgorandLogWatchConf struct {
	Path string
}

type AlgorandLogWatch struct {
	AlgorandLogWatchConf
	Watch

	FileNotifyWatch Watcher
	fileWatchCh     chan interface{}

	lastFilePos int64 //todo comes from agent config
}

func NewAlgorandLogWatch(conf AlgorandLogWatchConf, fileNotifyWatch Watcher) *AlgorandLogWatch {
	w := new(AlgorandLogWatch)
	w.AlgorandLogWatchConf = conf
	w.Watch = NewWatch()
	w.FileNotifyWatch = fileNotifyWatch
	w.fileWatchCh = make(chan interface{}, 1)
	w.lastFilePos = 0
	return w
}

func (w *AlgorandLogWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	// Determine file size
	info, err := os.Stat(w.Path)
	if err != nil {
		log.Errorf("[AlgorandLogWatch] Failed to read log file size: %v\n", err)
		return //todo
	}
	w.lastFilePos = info.Size()

	if w.FileNotifyWatch == nil {
		w.FileNotifyWatch = NewFileNotifyWatch(FileNotifyWatchConf{
			FileWatchConf:  FileWatchConf{Path: w.Path},
			AllowNotExists: false,
			Ops:            fsnotify.Write,
		})
	}
	w.FileNotifyWatch.Subscribe(w.fileWatchCh)
	Start(w.FileNotifyWatch)

	go w.handleFileChange()

}

func (w *AlgorandLogWatch) handleFileChange() {
	for {
		select {
		case _ = <-w.fileWatchCh:
			// START GENERIC LOG WATCHER

			file, err := os.OpenFile(w.Path, os.O_RDONLY, 077)
			if err != nil {
				panic(err)
			}

			stat, err := file.Stat()
			if err != nil {
				panic(err)
			}
			fileEnd := stat.Size()

			var data = make([]byte, fileEnd-w.lastFilePos-1)
			_, err = file.ReadAt(data, w.lastFilePos)
			if err != nil {
				panic(err)
			}
			w.lastFilePos = fileEnd - 1

			var jsonResult map[string]interface{}
			err = json.Unmarshal(data, &jsonResult)
			if err != nil {
				log.Tracef("[AlgorandLogWatch] %s\n", string(data))
				log.Errorf("[AlgorandLogWatch] Failed to unmarshal log line: %v\n", err)
				continue
			}

			// END GENERIC JSON LOG WATCHER

			if val, ok := jsonResult["Type"]; ok {
				_ = val
				if val == "RoundConcluded" {
					log.Info("[AlgorandLogWatch] Round Concluded Event")
				}
			}

		case <-w.StopKey:
			return

		}
	}
}
