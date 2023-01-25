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
	"bufio"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/stretchr/testify/require"
)

type MockJournal struct {
	testdataFile *os.File
	scanner      *bufio.Scanner
	failAfter    time.Duration
	failAt       time.Time
	failOnce     *sync.Once
}

func NewMockJournal(testdata string, fail bool) (*MockJournal, error) {
	f, err := os.Open(testdata)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)

	return &MockJournal{
		testdataFile: f,
		scanner:      scanner,
		failAt:       time.Now().Add(100 * time.Millisecond),
		failOnce:     &sync.Once{},
	}, nil
}

func (m *MockJournal) AddMatch(_ string) error {
	return nil
}

func (m *MockJournal) AddDisjunction() error {
	return nil
}

func (m *MockJournal) SeekTail() error {
	return nil
}

func (m *MockJournal) Wait(d time.Duration) int {
	return 0
}

func (m *MockJournal) Next() (uint64, error) {
	fail := false
	if time.Now().After(m.failAt) {
		m.failOnce.Do(func() {
			fail = true
		})
		if fail {
			fail = false
			return 0, errors.New("random next error")
		}
	}

	return 1, nil
}

func (m *MockJournal) GetEntry() (*sdjournal.JournalEntry, error) {
	fail := false
	if time.Now().After(m.failAt) {
		m.failOnce.Do(func() {
			fail = true
		})
		if fail {
			fail = false
			return nil, errors.New("random get entry error")
		}
	}

	if m.scanner.Scan() {
		text := m.scanner.Text()
		return &sdjournal.JournalEntry{
			Fields: map[string]string{sdjournal.SD_JOURNAL_FIELD_MESSAGE: text},
		}, nil
	}

	if err := m.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, nil
}

func (m *MockJournal) Close() error {
	if m.testdataFile != nil {
		return m.testdataFile.Close()
	}

	return nil
}

func TestJournalLogWatch_Events(t *testing.T) {
	tests := []struct {
		name string
		fail bool
	}{
		{name: "without errors", fail: false},
		{name: "with random errors", fail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mj, err := NewMockJournal("./testdata/journal_logs.json", tt.fail)
			require.Nil(t, err)

			conf := JournaldLogWatchConf{
				UnitName: "foobar",
				Journal:  mj,
				Events: map[string]model.FromContext{
					"OnVoting": new(onVoting),
				},
			}
			w, err := NewJournaldLogWatch(conf)
			require.Nil(t, err)

			ch := make(chan interface{}, 100)
			gotEvs := []*model.Message{}

			done := make(chan bool, 1)
			go func() {
				for {
					select {
					case ev := <-ch:
						event, ok := ev.(*model.Message)
						require.True(t, ok)

						gotEvs = append(gotEvs, event)
					case <-time.After(time.Second):
						done <- true
						return
					}
				}
			}()
			w.Subscribe(ch)

			w.StartUnsafe()
			defer w.Stop()

			select {
			case <-done:
				onVotingCnt := 0
				logMissingCnt := 0
				logFoundCnt := 0
				for _, ev := range gotEvs {
					switch ev.Name {
					case "OnVoting":
						onVotingCnt++
					case "agent.node.log.missing":
						logMissingCnt++
					case "agent.node.log.found":
						logFoundCnt++
					}
				}
				require.Equal(t, 5, onVotingCnt)
				require.Equal(t, 1, logMissingCnt)
				require.Equal(t, 1, logFoundCnt)
			case <-time.After(10 * time.Second):
				t.Error("timeout waiting for event collection goroutine finish")
			}
		})
	}
}

func TestJournalLogWatch_PendingStart(t *testing.T) {
	mj, err := NewMockJournal("./testdata/journal_logs.json", false)
	require.Nil(t, err)

	conf := JournaldLogWatchConf{
		PendingStartInterval: time.Millisecond,
		UnitName:             "foobar",
		Journal:              mj,
		Events: map[string]model.FromContext{
			"OnVoting": new(onVoting),
		},
	}

	w, err := NewJournaldLogWatch(conf)
	require.Nil(t, err)

	ch := make(chan interface{}, 100)
	gotEvs := []*model.Message{}
	done := make(chan bool)
	go func() {
		for {
			select {
			case ev := <-ch:
				event, ok := ev.(*model.Message)
				require.True(t, ok)

				gotEvs = append(gotEvs, event)
			case <-time.After(time.Second):
				done <- true
				return
			}
		}
	}()

	go func() {
		w.PendingStart(ch)
	}()

	select {
	case <-done:
		w.Stop()

		onVotingCnt := 0
		logMissingCnt := 0
		logFoundCnt := 0
		for _, ev := range gotEvs {
			switch ev.Name {
			case "OnVoting":
				onVotingCnt++
			case "agent.node.log.missing":
				logMissingCnt++
			case "agent.node.log.found":
				logFoundCnt++
			}
		}
		require.Equal(t, 5, onVotingCnt)
		require.Equal(t, 1, logMissingCnt)
		require.Equal(t, 1, logFoundCnt)
	case <-time.After(10 * time.Second):
		t.Error("timeout waiting for event collection goroutine finish")
	}
}
