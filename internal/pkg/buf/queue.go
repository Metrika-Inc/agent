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
	"container/heap"
	"time"

	"go.uber.org/zap"
)

// An Item is something we manage in a priority queue.
type Item struct {
	Priority
	Timestamp int64
	Data      interface{}
}

// A priorityQueue implements heap.Interface and holds Items.
type priorityQueue struct {
	items []Item
	ttl   time.Duration
}

func (pq priorityQueue) Len() int { return len(pq.items) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq.items[i].Timestamp < pq.items[j].Timestamp
}

func (pq priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

func (pq priorityQueue) Peek() interface{} {
	if len(pq.items) > 0 {
		return pq.items[0]
	}

	return nil
}

func (pq *priorityQueue) Push(x interface{}) {
	itemQ := x.(Item)
	pq.items = append(pq.items, itemQ)
}

func (pq *priorityQueue) Pop() interface{} {
	if len(pq.items) == 0 {
		return nil
	}

	old := pq.items
	n := len(old)
	itemQ := old[n-1]
	pq.items = old[0 : n-1]

	if len(pq.items) == 0 {
		pq.items = nil
	}

	return itemQ
}

func (pq *priorityQueue) DrainExpired() {
	for pq.Len() > 0 {
		v := pq.Peek()
		if v == nil {
			return
		}

		item, ok := v.(Item)
		if !ok {
			zap.S().Warnf("unknown queue item type peeked %T", item)

			return
		}

		now := time.Now()
		if pq.ttl > 0 && now.Sub(time.UnixMilli(item.Timestamp)) > pq.ttl { // drop
			v := heap.Pop(pq)
			item, ok := v.(Item)
			if !ok {
				zap.S().Warnf("unknown queue item type popped %T", item)

				return
			}

			continue
		}

		break
	}

	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Priority denotes queue priority
type Priority uint8

const (
	low Priority = iota
	med
	high
)

type multiQueue []*priorityQueue

func (m multiQueue) Len() int {
	cnt := 0
	for i := 0; i < len(m); i++ {
		cnt += m[i].Len()
	}

	return cnt
}

func (m multiQueue) Push(x interface{}) {
	itemQ, ok := x.(Item)
	if !ok {
		zap.S().Warnf("unknown queue item type %T", itemQ)

		return
	}

	q := m[min(int(itemQ.Priority), len(m)-1)]
	heap.Push(q, x)
}

func (m multiQueue) DrainExpired() {
	for i := 0; i < len(m); i++ {
		m[i].DrainExpired()
	}
}

func (m multiQueue) Pop() interface{} {
	for i := len(m) - 1; i >= 0; i-- {
		for m[i].Len() > 0 {
			v := heap.Pop(m[i])
			item, ok := v.(Item)
			if !ok {
				zap.S().Warnf("unknown queue item type %T", item)

				return nil
			}

			return item
		}
	}

	return nil
}

func newPriorityQueue(ttl time.Duration) *priorityQueue {
	return &priorityQueue{
		items: []Item{},
		ttl:   ttl,
	}
}

func newPriorityQueueN(ttl time.Duration, n int) []*priorityQueue {
	queues := []*priorityQueue{}
	for i := 0; i < n; i++ {
		q := newPriorityQueue(ttl)
		heap.Init(q)
		queues = append(queues, q)
	}
	return queues
}

func newMultiQueue(queues ...*priorityQueue) multiQueue {
	m := multiQueue{}
	for _, q := range queues {
		m = append(m, q)
	}

	return m
}
