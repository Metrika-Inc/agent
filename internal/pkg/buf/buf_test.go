package buf

import (
	"math/rand"
	"testing"
	"time"
	"unsafe"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
)

var (
	defaultMaxBufferBytes = uint(1024 * 1024) // 1MB
	testItemSz            = 152               // 160Bytes
)

func newTestItem(priority uint, m *model.Message) Item {
	return Item{
		Timestamp: time.Now().UnixMilli(),
		Priority:  Priority(priority),
		Bytes:     uint(unsafe.Sizeof(Item{})) + m.Bytes(),
	}
}

func newTestMetric(timestamp int64) model.Message {
	return model.Message{
		Name:  "heap-test",
		Value: &model.Message_MetricFamily{MetricFamily: &model.MetricFamily{Name: "foobar"}},
	}
}

func newTestItemBatch(n int) ItemBatch {
	got := make(ItemBatch, 0, n)
	for i := 0; i < n; i++ {
		metric := model.Message{Name: "heap-test"}
		got = append(got, newTestItem(0, &metric))
	}

	return got
}

func copyMetrics(m ItemBatch) ItemBatch {
	exp := make(ItemBatch, len(m))
	copy(exp, m)
	return exp
}

func TestPriorityBufferInsert(t *testing.T) {
	pb := NewPriorityBuffer(defaultMaxBufferBytes, 1*time.Hour)

	m := newTestItemBatch(1)
	_, err := pb.Insert(m...)
	require.NoError(t, err)
}

func TestPriorityBufferInsertMultiOutOfOrder(t *testing.T) {
	pb := NewPriorityBuffer(defaultMaxBufferBytes, time.Duration(0))

	m := newTestItemBatch(10)
	exp := copyMetrics(m)

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(m), func(i, j int) { m[i], m[j] = m[j], m[i] })
	_, err := pb.Insert(m...)

	got, _, err := pb.Get(10)
	require.NoError(t, err)

	require.Equal(t, exp, got)
	require.NoError(t, err)
}

func TestPriorityBufferGet(t *testing.T) {
	pb := NewPriorityBuffer(defaultMaxBufferBytes, time.Duration(0))

	m := newTestItemBatch(1)
	exp := copyMetrics(m)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	got, _, err := pb.Get(1)
	require.NoError(t, err)

	require.Equal(t, exp, got)
}

func TestPriorityBufferGetMulti(t *testing.T) {
	pb := NewPriorityBuffer(defaultMaxBufferBytes, time.Duration(0))

	m := newTestItemBatch(5)
	exp := copyMetrics(m)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	got, _, err := pb.Get(5)
	require.NoError(t, err)

	require.Equal(t, exp, got)
}

func TestPriorityBufferLen(t *testing.T) {
	pb := NewPriorityBuffer(defaultMaxBufferBytes, time.Duration(0))

	m := newTestItemBatch(5)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	require.Equal(t, 5, pb.Len())

	_, _, err = pb.Get(4)
	require.NoError(t, err)

	require.Equal(t, 1, pb.Len())

	_, _, err = pb.Get(1)
	require.NoError(t, err)

	require.Equal(t, 0, pb.Len())
}

func TestPriorityBufferSortInvariant(t *testing.T) {
	batch := ItemBatch{
		{Priority: Priority(low), Timestamp: 0},
		{Priority: Priority(med), Timestamp: 0},
		{Priority: Priority(low), Timestamp: 1},
		{Priority: Priority(high), Timestamp: 1},
		{Priority: Priority(low), Timestamp: 2},
		{Priority: Priority(high), Timestamp: 0},
		{Priority: Priority(med), Timestamp: 2},
		{Priority: Priority(med), Timestamp: 1},
	}

	exp := ItemBatch{
		{Priority: Priority(high), Timestamp: 0},
		{Priority: Priority(high), Timestamp: 1},
		{Priority: Priority(med), Timestamp: 0},
		{Priority: Priority(med), Timestamp: 1},
		{Priority: Priority(med), Timestamp: 2},
		{Priority: Priority(low), Timestamp: 0},
		{Priority: Priority(low), Timestamp: 1},
		{Priority: Priority(low), Timestamp: 2},
	}

	pb := NewPriorityBuffer(1000, time.Duration(0))

	_, err := pb.Insert(batch...)
	require.NoError(t, err)

	got, _, err := pb.Get(len(batch))
	require.NoError(t, err)

	require.Equal(t, exp, got)
}

// TestPriorityBufferInsertMaxSize uses:
// - a buffer with max size that would only fit one test item
// and checks:
// - an error is returned when a batch exceeds buffer max size
// - full batch is rejected if it exceeds buffer max size
func TestPriorityBufferInsertMaxSize(t *testing.T) {
	pb := NewPriorityBuffer(200, time.Duration(0))
	m := newTestItemBatch(5)

	n, err := pb.Insert(m...)
	require.Error(t, err)

	require.Equal(t, 0, int(n))
}

// TestPriorityBufferBytes uses:
// - a buffer with max size that fits exactly 5 test items
// and checks:
// - buffer size in bytes is equal to buffer's max size
func TestPriorityBufferBytes(t *testing.T) {
	sz := uint(testItemSz * 5)
	pb := NewPriorityBuffer(sz, time.Duration(0))
	m := newTestItemBatch(5)

	_, err := pb.Insert(m...)
	require.NoError(t, err)

	require.Equal(t, int(sz), int(pb.Bytes()))
}

func TestItemBatchAdd(t *testing.T) {
	b := new(ItemBatch)

	metric := newTestMetric(0)
	item := newTestItem(0, &metric)
	b.Add(item)

	require.Equal(t, 1, len(*b))
}

func TestItemBatchClear(t *testing.T) {
	b := new(ItemBatch)

	metric := newTestMetric(0)
	item := newTestItem(0, &metric)
	b.Add(item)

	require.Equal(t, 1, len(*b))

	b.Clear()

	require.Equal(t, 0, len(*b))
}
