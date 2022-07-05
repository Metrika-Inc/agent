package buf

import (
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
)

func TestPriorityQueuePop(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, 1, model.Message{}})
	q.Push(Item{0, 1, 1, model.Message{}})

	require.Equal(t, len(q.items), 2)

	got := q.Pop()
	require.IsType(t, Item{}, got)
	require.Equal(t, len(q.items), 1)
	require.Equal(t, Item{0, 1, 1, model.Message{}}, got)

	got = q.Pop()
	require.IsType(t, Item{}, got)
	require.Equal(t, len(q.items), 0)
	require.Equal(t, Item{0, 0, 1, model.Message{}}, got)
}

func TestPriorityQueuePush(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, 1, model.Message{}})
	q.Push(Item{0, 1, 1, model.Message{}})

	require.Equal(t, 2, q.Len())
}

func TestPriorityQueuePeek(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, 1, model.Message{}})
	q.Push(Item{0, 1, 1, model.Message{}})

	require.Equal(t, 2, q.Len())

	require.IsType(t, Item{}, q.Peek())
}

func TestPriorityQueueDrainExpired(t *testing.T) {
	ttl := 1 * time.Millisecond
	q := newPriorityQueue(ttl)
	q.Push(Item{0, 0, 1, model.Message{}})
	q.Push(Item{0, 1, 1, model.Message{}})

	require.Equal(t, 2, q.Len())

	<-time.After(ttl)
	q.DrainExpired()

	require.Equal(t, 0, q.Len())
}

func TestPriorityQueueDrainExpiredSortInvariant(t *testing.T) {
	ttl := 30 * time.Minute

	q := newPriorityQueue(ttl)
	tsExpired := time.Now().Add(-1 * time.Hour).UnixMilli()
	tsRecent := time.Now().Add(-10 * time.Minute).UnixMilli()
	mExpired := Item{0, tsExpired, 1, model.Message{}}
	mRecent := Item{0, tsRecent, 1, model.Message{}}
	q.Push(mExpired)
	q.Push(mRecent)

	require.Equal(t, 2, q.Len())
	q.DrainExpired()

	require.Equal(t, 1, q.Len())

	gotItem := q.Pop()
	require.IsType(t, Item{}, gotItem)

	item := gotItem.(Item)
	require.Equal(t, item, mRecent)
}

func TestPriorityQueueDrainExpiredRegression(t *testing.T) {
	q := newPriorityQueue(1 * time.Hour)
	q.Push(Item{0, time.Now().UnixMilli(), 1, model.Message{}})
	q.Push(Item{0, time.Now().UnixMilli(), 1, model.Message{}})
	q.DrainExpired()

	require.Equal(t, 2, q.Len())
}

func TestMultiQueuePop(t *testing.T) {
	q := newMultiQueue(newPriorityQueueN(time.Duration(0), 3)...)
	item1 := Item{0, 2, 1, model.Message{}}
	item2 := Item{1, 2, 1, model.Message{}}
	item3 := Item{2, 2, 1, model.Message{}}

	q.Push(item1)
	q.Push(item2)
	q.Push(item3)

	got, _ := q.Pop()
	require.Equal(t, 2, q.Len())
	require.Equal(t, Item{2, 2, 1, model.Message{}}, got)

	got, _ = q.Pop()
	require.Equal(t, 1, q.Len())
	require.Equal(t, Item{1, 2, 1, model.Message{}}, got)

	got, _ = q.Pop()
	require.Equal(t, 0, q.Len())
	require.Equal(t, Item{0, 2, 1, model.Message{}}, got)

	got, _ = q.Pop()
	require.Nil(t, got)
}
