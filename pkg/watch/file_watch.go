package watch

import (
	"github.com/fsnotify/fsnotify"
)

type FileNotifyWatchOld struct {
	Watch
	FileWatchConf

	watcher *fsnotify.Watcher
}

//func NewFileNotifyWatchOld(conf FileWatchConf) *FileNotifyWatchOld {
//	w := new(FileNotifyWatch)
//	w.Watch = NewWatch()
//	w.FileWatchConf = conf
//
//	var err error
//	w.watcher, err = fsnotify.NewWatcher()
//	if err != nil {
//		panic(err)
//	}
//	go func() {
//		for {
//			select {
//			case event, ok := <-w.watcher.Events:
//				if !ok {
//					return
//				}
//				log.Println("event:", event)
//				if event.Op&fsnotify.Write == fsnotify.Write {
//					log.Println("modified file:", event.Name)
//				}
//			case err, ok := <-w.watcher.Errors:
//				if !ok {
//					return
//				}
//				log.Println("error:", err)
//			}
//		}
//	}()
//
//	err = w.watcher.Add(w.Path)
//
//	return w
//}

//func main() {
//	watcher, err := fsnotify.NewWatcher()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer watcher.Close()
//
//	done := make(chan bool)
//	go func() {
//		for {
//			select {
//			case event, ok := <-watcher.Events:
//				if !ok {
//					return
//				}
//				log.Println("event:", event)
//				if event.Op&fsnotify.Write == fsnotify.Write {
//					log.Println("modified file:", event.Name)
//				}
//			case err, ok := <-watcher.Errors:
//				if !ok {
//					return
//				}
//				log.Println("error:", err)
//			}
//		}
//	}()
//
//	err = watcher.Add("/tmp/foo")
//	if err != nil {
//		log.Fatal(err)
//	}
//	<-done
//}
