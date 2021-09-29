package main

import (
	"agent/pkg/watch"
	"fmt"
	"time"
)

func main() {
	fmt.Println("Hello, Agent!")

	//w := watch.NewTimerWatch(watch.TimerWatchConf{
	//	Interval: 1 * time.Second,
	//})

	w := watch.NewFileReadWatch(watch.FileReadWatchConf{
		FileWatchConf: watch.FileWatchConf{
			Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/metrika-agent/pkg/watch/file_watch.go",
		},
		Interval: time.Second,
	}, nil)

	ch := make(chan interface{}, 1)
	go func() {
		for {
			value := <-ch
			fmt.Println("Emitted", string(value.([]byte)))
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
