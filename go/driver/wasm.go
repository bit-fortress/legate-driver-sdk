//go:build tinygo

package driver

import "unsafe"

var allocations = map[uint32][]byte{}

// Alloc reserves guest memory for a Host-provided ABI message.
func Alloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	buffer := make([]byte, size)
	pointer := uint32(uintptr(unsafe.Pointer(&buffer[0])))
	allocations[pointer] = buffer
	return pointer
}

// Free releases an exact live allocation. Invalid, interior, mismatched, and
// already-freed buffers are ignored without touching guest memory.
func Free(pointer uint32, size uint32) {
	buffer, ok := allocations[pointer]
	if !ok || uint32(len(buffer)) != size {
		return
	}
	delete(allocations, pointer)
}

// Invoke reads one ABI request, invokes handler, and returns packed
// output-pointer/output-length as required by Legate ABI v1.
func Invoke(pointer uint32, length uint32, handler func([]byte) []byte) uint64 {
	if handler == nil {
		return 0
	}
	var input []byte
	if pointer != 0 || length != 0 {
		buffer, ok := allocations[pointer]
		if !ok || uint32(len(buffer)) != length {
			return 0
		}
		input = append([]byte(nil), buffer...)
	}
	output := handler(input)
	if len(output) == 0 {
		return 0
	}
	if uint64(len(output)) > uint64(^uint32(0)) {
		return 0
	}
	outputPointer := Alloc(uint32(len(output)))
	if outputPointer == 0 {
		return 0
	}
	copy(allocations[outputPointer], output)
	return uint64(outputPointer)<<32 | uint64(uint32(len(output)))
}
