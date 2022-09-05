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
	"sync"
	"time"

	"go.uber.org/zap"
)

// ItemBatch batch of buffered items.
type ItemBatch []Item

// Add adds a message to the batch.
func (a *ItemBatch) Add(msg Item) {
	*a = append(*a, msg)
}

// Clear resets the backing slice length
func (a *ItemBatch) Clear() {
	*a = (*a)[:0]
}

// PriorityBuffer buffer backed by a multi-heap struct
type PriorityBuffer struct {
	q   multiQueue
	mu  *sync.RWMutex
	ttl time.Duration
}

// Insert inserts one or more items to the buffer.
func (p *PriorityBuffer) Insert(ms ...Item) error {
	t := time.Now()
	defer func() {
		bufferInsertDuration.Observe(time.Since(t).Seconds())
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, m := range ms {
		p.q.Push(m)
	}

	return nil
}

// Get returns a batch with at most n items and deletes them from the buffer.
func (p *PriorityBuffer) Get(n int) (ItemBatch, error) {
	t := time.Now()
	defer func() {
		bufferGetDuration.Observe(time.Since(t).Seconds())
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

// Len returns the number of buffered items.
func (p *PriorityBuffer) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, q := range p.q {
		total += q.Len()
	}

	return total
}

// NewPriorityBuffer PriorityBuffer constructor. TTL is used
// to discard buffered data that were buffered for too long.
func NewPriorityBuffer(ttl time.Duration) *PriorityBuffer {
	p := PriorityBuffer{
		mu:  new(sync.RWMutex),
		ttl: ttl,
	}

	p.q = newMultiQueue(newPriorityQueueN(ttl, int(high)+1)...)

	return &p
}
