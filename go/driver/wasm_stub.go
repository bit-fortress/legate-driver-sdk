//go:build !tinygo

package driver

// Alloc is only available in TinyGo wasm builds.
func Alloc(uint32) uint32 {
	panic("driver.Alloc requires TinyGo wasm")
}

// Free is only available in TinyGo wasm builds.
func Free(uint32, uint32) {
	panic("driver.Free requires TinyGo wasm")
}

// Invoke is only available in TinyGo wasm builds.
func Invoke(uint32, uint32, func([]byte) []byte) uint64 {
	panic("driver.Invoke requires TinyGo wasm")
}
