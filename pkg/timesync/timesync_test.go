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

package timesync

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/stretchr/testify/require"
)

func TestTimeSync_QueryNTP(t *testing.T) {
	t.Run("QueryNTP/success", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "1000", 1)
		ts.queryNTP = mockQueryNTP
		err := ts.QueryNTP()
		require.NoError(t, err)
		require.Equal(t, time.Millisecond*1000, ts.Offset())
	})
	t.Run("QueryNTP/failure", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "notInteger", 1)
		ts.queryNTP = mockQueryNTP
		ts.delta = 1234
		err := ts.QueryNTP()
		require.Error(t, err)
		require.Equal(t, time.Duration(1234), ts.Offset())
	})
}

func TestTimeSync(t *testing.T) {
	t.Run("Start/tick_interval_and_stop", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "", 1)
		ts.SetSyncInterval(25 * time.Millisecond)
		var tickCounter testCounter
		ts.queryNTP = func(s string) (*ntp.Response, error) {
			tickCounter.increment()
			return &ntp.Response{}, nil
		}
		ts.Start(nil)
		// 3 ticks expected
		<-time.After(55 * time.Millisecond)
		ts.Stop()
		ts.waitForDone()
		<-time.After(30 * time.Millisecond)
		require.Equal(t, 2, tickCounter.get())
	})
	t.Run("Start/stop_on_graceful_exit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		ts := NewTimeSync(ctx, "", 1)
		ts.SetSyncInterval(25 * time.Millisecond)
		var tickCounter testCounter
		ts.queryNTP = func(s string) (*ntp.Response, error) {
			tickCounter.increment()
			return &ntp.Response{}, nil
		}
		ts.Start(nil)
		// 3 ticks expected
		<-time.After(55 * time.Millisecond)
		cancel()
		ts.waitForDone()
		<-time.After(30 * time.Millisecond)
		require.Equal(t, 2, tickCounter.get())
	})
	t.Run("Start/start_twice", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "", 1)
		ts.SetSyncInterval(25 * time.Millisecond)
		var tickCounter testCounter
		ts.queryNTP = func(s string) (*ntp.Response, error) {
			tickCounter.increment()
			return nil, errors.New("error does not affect start flow")
		}
		ts.Start(nil)
		// 3 ticks expected
		<-time.After(55 * time.Millisecond)
		require.Equal(t, 2, tickCounter.get())
		ts.SetSyncInterval(10 * time.Millisecond)
		ts.Start(nil)
		<-time.After(35 * time.Millisecond)
		ts.Stop()
		<-time.After(11 * time.Millisecond)
		require.Equal(t, 5, tickCounter.get())
	})
	t.Run("Start/sync_now", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "", 1)
		ts.SetSyncInterval(100 * time.Millisecond)
		var tickCounter testCounter
		ts.queryNTP = func(s string) (*ntp.Response, error) {
			tickCounter.increment()
			return &ntp.Response{}, nil
		}
		ts.Start(nil)
		<-time.After(50 * time.Millisecond)
		err := ts.SyncNow()
		require.NoError(t, err)
		<-time.After(80 * time.Millisecond)
		ts.Stop()
		ts.waitForDone()
		require.Equal(t, 1, tickCounter.get())
	})
	t.Run("Start/check_default_interval", func(t *testing.T) {
		ts := NewTimeSync(context.Background(), "", 1)
		ts.Start(nil)
		<-time.After(15 * time.Millisecond)
		ts.Stop()
		require.Equal(t, defaultSyncInterval, ts.interval)
	})
}

func TestTimeSync_Now(t *testing.T) {
	allowedDelta := time.Millisecond * 5
	testcases := []struct {
		offset time.Duration
		adjust bool
	}{
		{offset: 5 * time.Second, adjust: true},
		{offset: 10 * time.Second, adjust: false},
	}

	for _, testcase := range testcases {
		ts := NewTimeSync(context.Background(), "", 1)
		ts.delta = testcase.offset
		ts.shouldAdjust = testcase.adjust
		expectedTime := time.Now()
		if testcase.adjust {
			expectedTime = expectedTime.Add(testcase.offset)
		}
		result := ts.Now()
		resultDelta := result.Sub(expectedTime)
		if resultDelta < 0 {
			resultDelta = -resultDelta
		}
		require.Less(t, resultDelta, allowedDelta)
	}
}

// mockQueryNTP shares the signature with ntp.Query
// 'host' value will be typecast to milliseconds
func mockQueryNTP(host string) (*ntp.Response, error) {
	delta, err := strconv.Atoi(host)
	if err != nil {
		return nil, errors.New("mockQueryNTP expected a number passed as host")
	}
	offset := time.Duration(delta) * time.Millisecond
	resp := &ntp.Response{
		Time:        time.Now().Add(offset),
		ClockOffset: offset,
	}
	return resp, nil
}

// testCounter is a thread-safe counter
// useful for unit testing tickers
type testCounter struct {
	i int
	sync.Mutex
}

func (t *testCounter) increment() {
	t.Lock()
	defer t.Unlock()
	t.i++
}

func (t *testCounter) get() int {
	t.Lock()
	defer t.Unlock()
	return t.i
}
