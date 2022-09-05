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

package buf

import (
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
)

func TestPriorityQueuePop(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, model.Message{}})
	q.Push(Item{0, 1, model.Message{}})

	require.Equal(t, len(q.items), 2)

	got := q.Pop()
	require.IsType(t, Item{}, got)
	require.Equal(t, len(q.items), 1)
	require.Equal(t, Item{0, 1, model.Message{}}, got)

	got = q.Pop()
	require.IsType(t, Item{}, got)
	require.Equal(t, len(q.items), 0)
	require.Equal(t, Item{0, 0, model.Message{}}, got)

	got = q.Pop()
	require.Nil(t, got)
	require.Nil(t, q.items)
}

func TestPriorityQueuePush(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, model.Message{}})
	q.Push(Item{0, 1, model.Message{}})

	require.Equal(t, 2, q.Len())
}

func TestPriorityQueuePeek(t *testing.T) {
	q := newPriorityQueue(1 * time.Minute)
	q.Push(Item{0, 0, model.Message{}})
	q.Push(Item{0, 1, model.Message{}})

	require.Equal(t, 2, q.Len())

	require.IsType(t, Item{}, q.Peek())
}

func TestPriorityQueueDrainExpired(t *testing.T) {
	ttl := 1 * time.Millisecond
	q := newPriorityQueue(ttl)
	q.Push(Item{0, 0, model.Message{}})
	q.Push(Item{0, 1, model.Message{}})

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
	mExpired := Item{0, tsExpired, model.Message{}}
	mRecent := Item{0, tsRecent, model.Message{}}
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
	q.Push(Item{0, time.Now().UnixMilli(), model.Message{}})
	q.Push(Item{0, time.Now().UnixMilli(), model.Message{}})
	q.DrainExpired()

	require.Equal(t, 2, q.Len())
}

func TestMultiQueuePop(t *testing.T) {
	q := newMultiQueue(newPriorityQueueN(time.Duration(0), 3)...)
	item1 := Item{0, 2, model.Message{}}
	item2 := Item{1, 2, model.Message{}}
	item3 := Item{2, 2, model.Message{}}

	q.Push(item1)
	q.Push(item2)
	q.Push(item3)

	got := q.Pop()
	require.Equal(t, 2, q.Len())
	require.Equal(t, Item{2, 2, model.Message{}}, got)

	got = q.Pop()
	require.Equal(t, 1, q.Len())
	require.Equal(t, Item{1, 2, model.Message{}}, got)

	got = q.Pop()
	require.Equal(t, 0, q.Len())
	require.Equal(t, Item{0, 2, model.Message{}}, got)

	got = q.Pop()
	require.Nil(t, got)
}
