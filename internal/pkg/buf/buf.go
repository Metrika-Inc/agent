package buf

import (
	"fmt"
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

func (a ItemBatch) Bytes() uint {
	var bytesDec uint
	for _, m := range a {
		bytesDec += m.Bytes
	}

	return bytesDec
}

type FullErr struct {
	sz uint
}

func (f *FullErr) Error() string {
	return fmt.Sprintf("max size reached %d", f.sz)
}

type PriorityBuffer struct {
	q        multiQueue
	mu       *sync.RWMutex
	ttl      time.Duration
	bytes    uint
	maxBytes uint
}

func (p *PriorityBuffer) Insert(ms ...Item) (uint, error) {
	t := time.Now()
	defer func() {
		BufferInsertDuration.Observe(time.Since(t).Seconds())
		BufferSize.Set(float64(p.Bytes()))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	prevSz := p.bytes

	p.bytes -= p.q.DrainExpired()

	var totalSz uint
	for _, m := range ms {
		totalSz += m.Bytes
	}

	if p.bytes+totalSz > p.maxBytes {
		return 0, &FullErr{p.bytes}
	}

	for _, m := range ms {
		p.q.Push(m)
		p.bytes += m.Bytes
	}

	return p.bytes - prevSz, nil
}

func (p *PriorityBuffer) Get(n int) (ItemBatch, uint, error) {
	t := time.Now()
	defer func() {
		BufferGetDuration.Observe(time.Since(t).Seconds())
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	res := make([]Item, 0, n+1)

	p.bytes -= p.q.DrainExpired()

	var batchSz uint
	for len(res) < n {
		pop, sz := p.q.Pop()
		batchSz += sz

		if pop == nil {
			break
		}

		itemQ, ok := pop.(Item)
		if !ok {
			zap.L().Sugar().Warnw("illegal item type", "item", pop)

			continue
		}

		res = append(res, itemQ)
		p.bytes -= sz
	}

	return res, batchSz, nil
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

func (p *PriorityBuffer) Bytes() uint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.bytes
}

func NewPriorityBuffer(maxBytes uint, ttl time.Duration) *PriorityBuffer {
	p := PriorityBuffer{
		mu:       new(sync.RWMutex),
		maxBytes: maxBytes,
		ttl:      ttl,
	}

	p.q = newMultiQueue(newPriorityQueueN(ttl, int(high)+1)...)

	return &p
}
