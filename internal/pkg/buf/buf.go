package buf

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

type ItemBatch []Item

func (a *ItemBatch) Add(msg Item) {
	*a = append(*a, msg)
}

func (a *ItemBatch) Clear() {
	*a = (*a)[:0]
}

type PriorityBuffer struct {
	q   multiQueue
	mu  *sync.RWMutex
	ttl time.Duration
}

func (p *PriorityBuffer) Insert(ms ...Item) error {
	t := time.Now()
	defer func() {
		BufferInsertDuration.Observe(time.Since(t).Seconds())
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, m := range ms {
		p.q.Push(m)
	}

	return nil
}

func (p *PriorityBuffer) Get(n int) (ItemBatch, error) {
	t := time.Now()
	defer func() {
		BufferGetDuration.Observe(time.Since(t).Seconds())
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	res := make([]Item, 0, n+1)

	for len(res) < n {
		pop := p.q.Pop()

		if pop == nil {
			break
		}

		itemQ, ok := pop.(Item)
		if !ok {
			zap.S().Warnw("illegal item type", "item", pop)

			continue
		}

		res = append(res, itemQ)
	}

	return res, nil
}

func (p *PriorityBuffer) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, q := range p.q {
		total += q.Len()
	}

	return total
}

func NewPriorityBuffer(ttl time.Duration) *PriorityBuffer {
	p := PriorityBuffer{
		mu:  new(sync.RWMutex),
		ttl: ttl,
	}

	p.q = newMultiQueue(newPriorityQueueN(ttl, int(high)+1)...)

	return &p
}
