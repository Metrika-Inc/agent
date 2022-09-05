// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watch

import (
	"time"
)

// *** TimerWatch ***

// TimerWatchConf TimerWatch configuration struct.
type TimerWatchConf struct {
	Interval time.Duration
}

// TimerWatch implements Watcher interface.
// Emits 0 periodically per a configured interval.
type TimerWatch struct {
	TimerWatchConf
	Watch
}

// NewTimerWatch timer watch constructor.
func NewTimerWatch(conf TimerWatchConf) *TimerWatch {
	w := new(TimerWatch)
	w.Watch = NewWatch()
	w.TimerWatchConf = conf

	if w.Interval < 1 {
		w.Log.Debug("Using default interval of one second since none was provided.")
		w.Interval = time.Second
	}

	return w
}

// StartUnsafe sets watch running state to true
// and starts the timer goroutine.
func (w *TimerWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	w.wg.Add(1)
	go w.timerLoop()
}

func (w *TimerWatch) timerLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-time.After(w.Interval):
			w.Emit(0)

		case <-w.StopKey:
			return
		}
	}
}
