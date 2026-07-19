//go:build tinygo

package driver

import "testing"

func TestWASMBuffersRequireExactLiveAllocation(t *testing.T) {
	pointer := Alloc(4)
	if pointer == 0 {
		t.Fatal("Alloc(4) returned zero")
	}
	copy(allocations[pointer], []byte("test"))

	if result := Invoke(pointer, 3, func([]byte) []byte { return []byte("bad") }); result != 0 {
		t.Fatalf("Invoke with mismatched length = %d, want 0", result)
	}
	if result := Invoke(pointer+1, 3, func([]byte) []byte { return []byte("bad") }); result != 0 {
		t.Fatalf("Invoke with interior pointer = %d, want 0", result)
	}

	Free(pointer, 3)
	if _, ok := allocations[pointer]; !ok {
		t.Fatal("mismatched Free released allocation")
	}

	packed := Invoke(pointer, 4, func(input []byte) []byte {
		if string(input) != "test" {
			t.Fatalf("input = %q", input)
		}
		return []byte("done")
	})
	outputPointer := uint32(packed >> 32)
	outputLength := uint32(packed)
	if outputLength != 4 || string(allocations[outputPointer]) != "done" {
		t.Fatalf("output = pointer %d length %d bytes %q", outputPointer, outputLength, allocations[outputPointer])
	}

	Free(outputPointer, outputLength)
	Free(pointer, 4)
	if result := Invoke(pointer, 4, func([]byte) []byte { return []byte("bad") }); result != 0 {
		t.Fatalf("Invoke after Free = %d, want 0", result)
	}
}

func TestWASMZeroBufferSemantics(t *testing.T) {
	if pointer := Alloc(0); pointer != 0 {
		t.Fatalf("Alloc(0) = %d, want 0", pointer)
	}
	called := false
	if result := Invoke(0, 0, func(input []byte) []byte {
		called = true
		if len(input) != 0 {
			t.Fatalf("zero input length = %d", len(input))
		}
		return nil
	}); result != 0 {
		t.Fatalf("zero Invoke = %d, want 0", result)
	}
	if !called {
		t.Fatal("zero Invoke did not call handler")
	}
	Free(0, 0)
}
