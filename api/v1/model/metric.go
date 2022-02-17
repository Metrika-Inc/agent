package model

import (
	"unsafe"
)

// Bytes estimates the total size of the metric in bytes.
func (m Message) Bytes() uint {
	return uint(unsafe.Sizeof(m)) + multipleOf8(len(m.Name)) + multipleOf8(len(m.Body)) + protoBytes(len(m.XXX_unrecognized))
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

// protoBytes calculates the value of XXX_unrecognized that is
// generated in every protobuf model.
// It takes the argument of len(XXX_unrecognized).
// Note: other values are automatically calculated in unsafe.Sizeof operation.
func protoBytes(unrecognizedLen int) uint {
	return uint(unrecognizedLen) + multipleOf8(unrecognizedLen)
}
