package buf

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestControllerDrainBatch uses:
// - a controller with a priority buffer of size bufSz,
//   n pre inserted metrics and batch size 1.
// - a goroutine to run the controller
// and after stopping the controller checks:
// - drain callback batch size is equal to MaxDrainBatchLen
func TestControllerDrainBatch(t *testing.T) {
	n := 5
	bufSz := uint(testItemSz * n)

	drainCh := make(chan ItemBatch, n)
	onDrain := func(b ItemBatch) error {
		drainCh <- b

		return nil
	}

	conf := ControllerConf{
		BufDrainFreq:        1 * time.Millisecond,
		BufLenLimit:         n,
		OnBufRemoveCallback: onDrain,
	}

	pb := NewPriorityBuffer(bufSz, time.Duration(0))
	m := newTestItemBatch(n)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	ctrl := NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()

	select {
	case b := <-drainCh:
		require.Len(t, b, conf.BufLenLimit)
		require.Equal(t, b, m)
		require.Len(t, drainCh, 0)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for drain callback")
	}

	pb = NewPriorityBuffer(bufSz, time.Duration(0))
	_, err = pb.Insert(m...)
	require.NoError(t, err)

	conf.BufLenLimit = 1
	ctrl = NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()
	require.Equal(t, 0, pb.Len())

	for i := 0; i < n; i++ {
		select {
		case b := <-drainCh:
			require.Len(t, b, conf.BufLenLimit)
			require.Equal(t, b[0], m[i])
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for drain callback")
		}
	}
}

func TestControllerDrainCallback(t *testing.T) {
	n := 5
	bufSz := uint(testItemSz * n)

	drainCh := make(chan ItemBatch, n)
	onDrain := func(b ItemBatch) error {
		drainCh <- b

		return nil
	}

	conf := ControllerConf{
		BufDrainFreq:        1 * time.Millisecond,
		BufLenLimit:         n,
		OnBufRemoveCallback: onDrain,
	}

	pb := NewPriorityBuffer(bufSz, time.Duration(0))
	m := newTestItemBatch(n)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	ctrl := NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()

	select {
	case b := <-drainCh:
		require.Equal(t, b, m)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for drain callback")
	}
	require.Equal(t, 0, pb.Len())
}

func TestControllerDrainCallbackErr(t *testing.T) {
	n := 5
	bufSz := uint(testItemSz*n) + uint(216)

	onDrain := func(b ItemBatch) error {
		return fmt.Errorf("drain test error")
	}

	conf := ControllerConf{
		BufDrainFreq:        1 * time.Millisecond,
		BufLenLimit:         n,
		OnBufRemoveCallback: onDrain,
	}

	pb := NewPriorityBuffer(bufSz, time.Duration(0))
	m := newTestItemBatch(n)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	ctrl := NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()
	require.Equal(t, n+1, pb.Len()) // +1 to compensate for agent.net.error event
}

// TestControllerDrain uses:
// - a controller with a priority buffer of size bufSz,
//   n pre inserted metrics and batch size 2*n.
// - a goroutine to run the controller
// and checks:
// - batch smaller than MaxDrainBatchLen is evicted periodically
func TestControllerDrain(t *testing.T) {
	n := 5
	bufSz := uint(testItemSz * n)

	drainCh := make(chan ItemBatch, n)
	onDrain := func(b ItemBatch) error {
		drainCh <- b

		return nil
	}

	conf := ControllerConf{
		BufDrainFreq:        1 * time.Millisecond,
		BufLenLimit:         2 * n,
		OnBufRemoveCallback: onDrain,
	}

	pb := NewPriorityBuffer(bufSz, time.Duration(0))
	m := newTestItemBatch(n)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	ctrl := NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()

	select {
	case b := <-drainCh:
		require.Equal(t, b, m)
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for drain callback")
	}
	require.Equal(t, 0, pb.Len())
}

func TestControllerClose(t *testing.T) {
	n := 5
	bufSz := uint(testItemSz * n)

	onDrain := func(b ItemBatch) error {
		return nil
	}

	conf := ControllerConf{
		BufDrainFreq:        1 * time.Millisecond,
		BufLenLimit:         1,
		OnBufRemoveCallback: onDrain,
	}

	pb := NewPriorityBuffer(bufSz, time.Duration(0))
	m := newTestItemBatch(n)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	ctrl := NewController(conf, pb)
	go ctrl.Start()
	<-time.After(100 * time.Millisecond)

	ctrl.Stop()

	assert.Equal(t, 0, pb.Len())
}
