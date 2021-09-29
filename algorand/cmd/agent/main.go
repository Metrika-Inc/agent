package main

import (
	"agent/pkg/watch"
	"fmt"
)

func main() {
	fmt.Println("Hello, Agent!")

	//timer := watch.NewTimerWatch(1 * time.Second)
	//timer.Start()

	fw := watch.NewFileNotifyWatch(watch.FileWatchConf{
		Path: "/Users/mattworzala/dev/sigmoidbell/node-agent/agent/pkg/watch/file_watch.go",
	})
	fw.Start()

	//pid := watch.NewPidWatch(nil)
	//pid.Start()

	//go func() {
	//	<-time.After(2 * time.Second)
	//
	//	timer.Interval = 50 * time.Millisecond
	//}()

	<-make(chan bool, 1)
}
