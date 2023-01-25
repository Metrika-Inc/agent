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
	"errors"
	"fmt"
	"time"

	"agent/api/v1/model"
	"agent/internal/pkg/global"

	"github.com/coreos/go-systemd/v22/sdjournal"
	"go.uber.org/zap"
)

// ErrJournaldLogWatchConf error indicating a watch configuration error
var ErrJournaldLogWatchConf = errors.New("journald log watch configuration error")

// Journal is an interface for agent access to sdjournal.Journal used for mocking purposes
type Journal interface {
	AddMatch(string) error
	AddDisjunction() error
	SeekTail() error
	Wait(time.Duration) int
	Next() (uint64, error)
	GetEntry() (*sdjournal.JournalEntry, error)
	Close() error
}

// JournaldLogWatchConf JournaldLogWatch configuration struct
type JournaldLogWatchConf struct {
	UnitName             string
	Events               map[string]model.FromContext
	PendingStartInterval time.Duration
	Journal              Journal
}

// JournaldLogWatch uses a dbus connection to monitor the status of
// a systemd service.
type JournaldLogWatch struct {
	JournaldLogWatchConf
	Watch

	watchCh    chan interface{}
	journal    Journal
	logMissing bool
}

// NewJournaldLogWatch JournaldLogWatch constructor
func NewJournaldLogWatch(conf JournaldLogWatchConf) (*JournaldLogWatch, error) {
	w := new(JournaldLogWatch)
	w.Watch = NewWatch()
	w.JournaldLogWatchConf = conf
	w.watchCh = make(chan interface{}, 1)
	w.logMissing = true

	if w.PendingStartInterval == 0 {
		w.PendingStartInterval = defaultPendingStartInterval
	}

	if len(w.UnitName) < 1 {
		return nil, ErrJournaldLogWatchConf
	}

	if err := w.resetJournal(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *JournaldLogWatch) resetJournal() error {
	var (
		journal Journal
		err     error
	)

	if w.Journal != nil {
		journal = w.Journal
	} else {
		journal, err = sdjournal.NewJournal()
		if err != nil {
			return err
		}

	}

	match := sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT + "=" + w.UnitName
	if err := journal.AddMatch(match); err != nil {
		return fmt.Errorf("journal add match error: %v", err.Error())
	}

	if err := journal.AddDisjunction(); err != nil {
		return fmt.Errorf("journal match OR error: %v", err.Error())
	}

	match = sdjournal.SD_JOURNAL_FIELD_SYSTEMD_USER_UNIT + "=" + w.UnitName
	if err := journal.AddMatch(match); err != nil {
		return fmt.Errorf("journal add match error: %v", err.Error())
	}

	if err := journal.SeekTail(); err != nil {
		return fmt.Errorf("journald seek tail error: %v", zap.Error(err))
	}

	w.journal = journal

	return nil
}

func (w *JournaldLogWatch) progressJournal() ([]byte, error) {
	n, err := w.journal.Next()
	if err != nil {
		return nil, err
	}

	if n < 1 {
		w.journal.Wait(time.Second)
		return nil, nil
	}

	entry, err := w.journal.GetEntry()
	if err != nil {
		return nil, err
	}

	if entry == nil || entry.Fields == nil {
		return nil, fmt.Errorf("got unexpected entry from journal (null or null fields): %v", entry)
	}

	v, ok := entry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE]
	if !ok {
		return nil, fmt.Errorf("journal entry without SD_JOURNAL_FIELD_MESSAGE field")
	}

	return []byte(v), nil
}

// StartUnsafe starts the goroutine for maintaining discovery and
// emitting events about a systemd service.
func (w *JournaldLogWatch) StartUnsafe() {
	w.Watch.StartUnsafe()

	w.wg.Add(1)
	go func() {
		w.wg.Done()

		for {
			if w.journal == nil {
				zap.S().Debug("resetting journal tailer")
				if err := w.resetJournal(); err != nil {
					zap.S().Errorw("error resetting journal tailer, retrying in 5s", zap.Error(err))
					time.Sleep(5 * time.Second)
					continue
				}
			}

			select {
			case <-w.StopKey:
				if err := w.journal.Close(); err != nil {
					zap.S().Errorw("error closing journal", zap.Error(err))
				}
				zap.S().Debug("closed journal log tailer")

				return
			default:
			}

			v, err := w.progressJournal()
			if err != nil {
				w.logMissing = true
				zap.S().Errorw("journal progress error, retrying in 5s", zap.Error(err))
				w.emitAgentNodeEvent(model.AgentNodeLogMissingName)

				w.journal = nil

				time.Sleep(5 * time.Second)

				continue
			}

			if v == nil || len(v) == 0 {
				continue
			}

			if w.logMissing {
				w.emitAgentNodeEvent(model.AgentNodeLogFoundName)
				zap.S().Info("journal log tailer recovered")
				w.logMissing = false
			}

			if string(v[0]) != "{" {
				continue
			}

			jsonMap, err := w.parseJSON(v)
			if err != nil {
				w.Log.Warnw("error parsing log line:", zap.Error(err), "msg", string(v))

				continue
			}

			w.emitNodeLogEvents(w.Events, jsonMap)
		}
	}()
}

// PendingStart waits until node type is determined and calls
// chain.LogWatchEnabled() to check if it should start up or not.
func (w *JournaldLogWatch) PendingStart(subscriptions ...chan<- interface{}) {
	ticker := time.NewTicker(w.PendingStartInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.StopKey:
			return
		case <-ticker.C:
			nodeType := global.BlockchainNode.NodeRole()
			if nodeType == "" {
				continue
			}

			log := w.Log.With("node_type", nodeType)
			if global.BlockchainNode.LogWatchEnabled() {
				if err := DefaultWatchRegistry.RegisterAndStart(w, subscriptions...); err != nil {
					log.Errorw("failed to register journal log watcher", zap.Error(err))
					return
				}
				log.Info("journal log watch started")
			} else {
				log.Info("journal log watch disabled - node type does not require logs to be watched")
			}
			return
		}
	}
}
