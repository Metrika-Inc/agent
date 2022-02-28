package timesync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/stretchr/testify/require"
)

func TestPlatformSync_RegisterAndCheck(t *testing.T) {
	t.Run("Unhealthy timestamp on 4th registration", func(t *testing.T) {
		p := &PlatformSync{RWMutex: &sync.RWMutex{}}

		// 1st registration, expect all timestamps equal
		ts := time.Now().Add(-100 * time.Millisecond).UnixNano()
		p.Register(ts)
		expectedTs := time.Unix(0, ts)
		require.Equal(t, expectedTs, p.firstTimestamp)
		require.Equal(t, expectedTs, p.prevTimestamp)
		require.Equal(t, expectedTs, p.currentTimestamp)
		// account for execution delay when calculating deltas
		require.Less(t, abs(100*time.Millisecond-p.firstDelta), time.Millisecond)
		require.Less(t, abs(100*time.Millisecond-p.prevDelta), time.Millisecond)
		require.Less(t, abs(100*time.Millisecond-p.currentDelta), time.Millisecond)

		require.True(t, p.Healthy())

		// 2nd registration, expect prev/current timestamp change
		ts2 := time.Now().Add(-50 * time.Millisecond).UnixNano()
		expectedTs2 := time.Unix(0, ts2)
		p.Register(ts2)
		require.Equal(t, expectedTs, p.firstTimestamp)
		require.Equal(t, expectedTs, p.prevTimestamp)
		require.Equal(t, expectedTs2, p.currentTimestamp)

		require.Less(t, abs(100*time.Millisecond-p.firstDelta), time.Millisecond)
		require.Less(t, abs(100*time.Millisecond-p.prevDelta), time.Millisecond)
		require.Less(t, abs(50*time.Millisecond-p.currentDelta), time.Millisecond)

		require.True(t, p.Healthy())

		// 3rd registration
		ts3 := time.Now().Add(150 * time.Millisecond).UnixNano()
		expectedTs3 := time.Unix(0, ts3)
		p.Register(ts3)
		require.Equal(t, expectedTs, p.firstTimestamp)
		require.Equal(t, expectedTs2, p.prevTimestamp)
		require.Equal(t, expectedTs3, p.currentTimestamp)

		require.Less(t, abs(100*time.Millisecond-p.firstDelta), time.Millisecond)
		require.Less(t, abs(50*time.Millisecond-p.prevDelta), time.Millisecond)
		require.Less(t, abs(150*time.Millisecond-p.currentDelta), time.Millisecond)

		require.True(t, p.Healthy())

		// 4th registration, increase the delta beyond threshhold
		ts4 := time.Now().Add(5500 * time.Millisecond).UnixNano()
		expectedTs4 := time.Unix(0, ts4)
		p.Register(ts4)
		require.Equal(t, expectedTs, p.firstTimestamp)
		require.Equal(t, expectedTs3, p.prevTimestamp)
		require.Equal(t, expectedTs4, p.currentTimestamp)

		require.Less(t, abs(100*time.Millisecond-p.firstDelta), time.Millisecond)
		require.Less(t, abs(150*time.Millisecond-p.prevDelta), time.Millisecond)
		require.Less(t, abs(5500*time.Millisecond-p.currentDelta), time.Millisecond)

		require.False(t, p.Healthy())
	})

	t.Run("Unhealthy on 1st registration", func(t *testing.T) {
		p := &PlatformSync{RWMutex: &sync.RWMutex{}}

		// 1st registration, expect all timestamps equal
		ts := time.Now().Add(-25000 * time.Millisecond).UnixNano()
		p.Register(ts)
		require.False(t, p.Healthy())
	})
}

func TestPlatformSync_LastDeltas(t *testing.T) {
	p := &PlatformSync{RWMutex: &sync.RWMutex{}}
	ts := time.Now().Add(-100 * time.Millisecond).UnixNano()
	p.Register(ts)
	ts2 := time.Now().Add(5500 * time.Millisecond).UnixNano()
	p.Register(ts2)
	d1, d2 := p.LastDeltas()
	require.Less(t, abs(p.prevDelta-d1), time.Millisecond)
	require.Less(t, abs(p.currentDelta-d2), time.Millisecond)

}

func TestAbs(t *testing.T) {
	testCases := []struct {
		input, expected time.Duration
	}{
		{input: 100, expected: 100},
		{input: -123, expected: 123},
		{input: 0, expected: 0},
		{input: -1, expected: 1},
	}
	for _, testCase := range testCases {
		result := abs(testCase.input)
		require.Equal(t, testCase.expected, result)
	}
}

func TestTrackTimestamps(t *testing.T) {
	// Test preparations
	queryFn := Default.queryNTP
	defer func() {
		Default.queryNTP = queryFn
	}()

	m := &sync.Mutex{}
	var called bool

	Default.queryNTP = func(s string) (*ntp.Response, error) {
		m.Lock()
		defer m.Unlock()
		called = true
		return &ntp.Response{}, nil
	}
	Default.Start(nil)

	<-time.After(10 * time.Millisecond)

	// test scenario
	ctx, cancel := context.WithCancel(context.Background())
	ch := TrackTimestamps(ctx)
	ch <- time.Now().UnixNano()
	<-time.After(10 * time.Millisecond)
	require.False(t, called)
	ch <- time.Now().Add(15 * time.Second).UnixNano()
	<-time.After(10 * time.Millisecond)
	cancel()
	m.Lock()
	defer m.Unlock()
	require.True(t, called)
}
