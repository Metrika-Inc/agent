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

// Buffer interface for reading and writing data to a buffer implementation.
type Buffer interface {
	// Insert inserts a variadic number of items to the backing store
	Insert(m ...Item) error

	// Get returns at most n items from the backing store
	Get(n int) (ItemBatch, error)

	// Len returns the number of items in the buffer
	Len() int
}
