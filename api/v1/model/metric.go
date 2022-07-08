package model

import (
	"unsafe"

	"google.golang.org/protobuf/proto"
)

// TODO: Remove when we migrate to monitoring runtime stats.
//
// Bytes estimates the total size of the metric in bytes.
func (m *Message) Bytes() uint {
	if m == nil {
		return uint(unsafe.Sizeof(m))
	}
	// FIXME: temporary fix for model restructuring work.
	body, err := proto.Marshal(m)
	if err != nil {
		return 0
	}

	// FIXME: Sizeof(m) might not work due to pointer ref; new model does not have XXX vars but has others.
	return uint(unsafe.Sizeof(*m)) + multipleOf8(len(m.Name)) + multipleOf8(len(body))
}

// multipleOf8 is a helper function for padding.
// It rounds up to the next multiple of 8.
// If length is zero, it returns zero.
func multipleOf8(length int) uint {
	if length == 0 {
		return 0
	}
	return uint((length + 7) & -8)
}
