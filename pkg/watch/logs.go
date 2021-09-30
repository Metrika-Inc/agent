package watch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"os"
)

type JsonLogWatchConf struct {
	Path string
}

type JsonLogWatch struct {
	JsonLogWatchConf
	Watch

	FileNotifyWatch Watcher
	fileWatchCh     chan interface{}

	file        *os.File
	lastFilePos int64

	partialString []byte
}

func NewJsonLogWatch(conf JsonLogWatchConf, fileNotifyWatch Watcher) *JsonLogWatch {
	w := new(JsonLogWatch)
	w.JsonLogWatchConf = conf
	w.Watch = NewWatch()
	w.FileNotifyWatch = fileNotifyWatch
	w.fileWatchCh = make(chan interface{}, 1)
	return w
}

func (w *JsonLogWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	// Check for missing path
	if w.Path == "" {
		panic("Missing required argument 'path'") //todo handle
	}

	// Open the file
	var err error
	w.file, err = os.OpenFile(w.Path, os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("[JsonLogWatch] Failed to open file '%s': %e\n", w.Path, err)
		return //todo need to retry here
	}

	// Read the starting length todo should come from agent cache and only use file size when there is no size present
	w.lastFilePos, err = GetEnd(w.file)
	if err != nil {
		log.Errorf("[JsonLogWatch] Failed to read file length of '%s': %e\n", w.Path, err)
		return //todo need to retry here
	}

	// Create and start file notifier
	if w.FileNotifyWatch == nil {
		w.FileNotifyWatch = NewFileNotifyWatch(FileNotifyWatchConf{
			FileWatchConf: FileWatchConf{Path: w.Path},
			Ops:           fsnotify.Write,
		})
	}
	w.FileNotifyWatch.Subscribe(w.fileWatchCh)
	Start(w.FileNotifyWatch)
	go w.handleFileChanges()
}

func (w *JsonLogWatch) Stop() {
	w.Watch.Stop()

	err := w.file.Close()
	if err != nil {
		log.Errorf("[JsonLogWatch] Failed to close target file '%s': %v\n", w.Path, err)
	}
}

func (w *JsonLogWatch) handleFileChanges() {
	for {
		select {
		case <-w.fileWatchCh:
			// Read data from last checkpoint to end
			data, err := ReadFromXToEnd(w.file, w.lastFilePos)
			if err != nil {
				log.Errorf("[JsonLogWatch] Failed to read new data from file '%s': %e\n", w.Path, err)
				continue
			}
			w.lastFilePos += int64(len(data))

			w.parseAndEmit(data)

		case <-w.StopKey:
			return

		}
	}
}

func (w *JsonLogWatch) parseAndEmit(buffer []byte) {
	for _, jsonString := range bytes.Split(buffer, []byte("\n")) {

		// Sanitization, incomplete (todo)
		if len(jsonString) < 2 {
			continue
		}

		var jsonResult map[string]interface{}
		err := json.Unmarshal(jsonString, &jsonResult)
		if err != nil {
			// Try to use existing partial
			if w.partialString != nil {
				attempt2 := append(w.partialString, jsonString...)
				w.partialString = nil
				err := json.Unmarshal(attempt2, &jsonResult)
				if err != nil {
					log.Errorf("[JsonLogWatch] Failed to unmarshall JSON data (%d): %v\n%#v\n", len(jsonString), err, string(jsonString))
					fmt.Printf(">>> %#v\n", string(buffer))
					continue
				}
			} else {
				w.partialString = jsonString
				//log.Errorf("[JsonLogWatch] Failed to unmarshall JSON data (%d): %v\n%#v\n", len(jsonString), err, string(jsonString))
				//fmt.Printf(">>> %#v\n", string(buffer))
				continue
			}
		}

		w.Emit(jsonResult)
	}
	//var start = 0
	//for end, b := range buffer {
	//	if b != '\n' && end != len(buffer)-1 {
	//		continue
	//	}
	//
	//	// We have reached a new line (or end of data), try to parse it as json
	//	jsonString := buffer[start : end]
	//	start = end + 1
	//
	//	// Sanitization, incomplete (todo)
	//	if len(jsonString) < 2 {
	//		continue
	//	}
	//
	//
	//}
}

// File utils (to be moved somewhere else?)

func GetEnd(f *os.File) (int64, error) {
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

func ReadFromXToEnd(f *os.File, x int64) ([]byte, error) {
	//todo take into account a max size to read before splitting it

	end, err := GetEnd(f)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, end-x-1)
	_, err = f.ReadAt(buffer, x)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}
