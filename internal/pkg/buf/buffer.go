package buf

type Buffer interface {
	// Insert inserts a variadic number of items to the backing store
	Insert(m ...Item) error

	// Get returns at most n items from the backing store
	Get(n int) (ItemBatch, error)

	// Len returns the number of items in the buffer
	Len() int
}
