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
	Bytes     uint
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

	return itemQ
}

func (pq *priorityQueue) DrainExpired() uint {
	expiredCnt := 0
	var claimSz uint

	for pq.Len() > 0 {
		v := pq.Peek()
		if v == nil {
			return 0
		}

		item, ok := v.(Item)
		if !ok {
			zap.S().Warnf("unknown queue item type peeked %T", item)

			return 0
		}

		now := time.Now()
		if pq.ttl > 0 && now.Sub(time.UnixMilli(item.Timestamp)) > pq.ttl { // drop
			v := heap.Pop(pq)
			item, ok := v.(Item)
			if !ok {
				zap.S().Warnf("unknown queue item type popped %T", item)

				return claimSz
			}

			claimSz += item.Bytes
			expiredCnt++

			continue
		}

		break
	}

	return claimSz
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

func (m multiQueue) DrainExpired() uint {
	var claimSz uint
	expiredCnt := 0

	for i := 0; i < len(m); i++ {
		claimSz += m[i].DrainExpired()
	}

	if expiredCnt > 0 {
		zap.S().Warnw("items in buffer expired", "expired_count", expiredCnt, "expired_bytes", claimSz)
	}

	return claimSz
}

func (m multiQueue) Pop() (interface{}, uint) {
	for i := len(m) - 1; i >= 0; i-- {
		for m[i].Len() > 0 {
			v := heap.Pop(m[i])
			item, ok := v.(Item)
			if !ok {
				zap.S().Warnf("unknown queue item type %T", item)

				return nil, 0
			}

			return item, item.Bytes
		}
	}

	return nil, 0
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
