package main

import (
	"agent/pkg/watch"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

func main() {
	fmt.Println("Hello, Agent!")

	//timer := watch.NewTimerWatch(1 * time.Second)
	//timer.Start()

	fw := watch.NewFileNotifyWatch(watch.FileNotifyWatchConf{
		FileWatchConf: watch.FileWatchConf{
			Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/sandbox/node_out.log",
		},
		Ops: fsnotify.Write,
	})
	watch.Start(fw)

	//pid := watch.NewPidWatch(nil)
	//pid.Start()

	//go func() {
	//	<-time.After(2 * time.Second)
	//
	//	timer.Interval = 50 * time.Millisecond
	//}()

	<-make(chan bool, 1)
}
