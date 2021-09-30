package main

import (
	"agent/pkg/watch"
	"fmt"
	log "github.com/sirupsen/logrus"
)

func main() {
	fmt.Println("Hello, Agent!")

	log.SetLevel(log.WarnLevel)

	//w := watch.NewTimerWatch(watch.TimerWatchConf{
	//	Interval: 1 * time.Second,
	//})

	//w := watch.NewFileReadWatch(watch.FileReadWatchConf{
	//	FileWatchConf: watch.FileWatchConf{
	//		Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/metrika-agent/pkg/watch/file_watch.go",
	//	},
	//	Interval: time.Second,
	//}, nil)

	//w := watch.NewDotPidWatch(watch.DotPidWatchConf{
	//	Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/file.pid",
	//})

	//w := NewAlgorandLogWatch(AlgorandLogWatchConf{
	//	Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/node_out.log",
	//}, nil)

	w := watch.NewJsonLogWatch(watch.JsonLogWatchConf{
		Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/node_out.log",
	}, nil)

	ch := make(chan interface{}, 1)
	go func() {
		for {
			value := <-ch
			log.Info("Emitted", value.(map[string]interface{}))
		}
	}()
	w.Subscribe(ch)

	watch.Start(w)

	//fw := watch.NewFileNotifyWatch(watch.FileNotifyWatchConf{
	//	FileWatchConf: watch.FileWatchConf{
	//		Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/node_out.log",
	//	},
	//	Ops: fsnotify.Write,
	//})
	//watch.Start(fw)

	//pid := watch.NewPidWatch(nil)
	//pid.Start()

	//go func() {
	//	<-time.After(2 * time.Second)
	//
	//	timer.Interval = 50 * time.Millisecond
	//}()

	<-make(chan bool, 1)
}
