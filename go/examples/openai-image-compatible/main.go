package main

import (
	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	edit "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/edits/v20260719"
	generation "github.com/bootun/legate-driver-sdk/go/protocol/openai/images/generations/v20260719"
)

var dispatcher = sdk.MustNewImageDispatcher(
	[]string{generation.Contract, edit.Contract},
	imageBinder{},
	generation.Register(generationHandler{}),
	edit.Register(editHandler{}),
)
var guest = sdk.NewImageGuest(dispatcher)

//export legate_alloc_v1
func legateAlloc(size uint32) uint32 { return sdk.Alloc(size) }

//export legate_free_v1
func legateFree(pointer uint32, size uint32) { sdk.Free(pointer, size) }

//export legate_bind_v1
func legateBind(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleBind)
}

//export legate_image_attempt_open_v1
func legateImageAttemptOpen(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleImageAttemptOpen)
}

//export legate_image_transform_buffered_response_v1
func legateImageTransformBufferedResponse(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleImageTransformBufferedResponse)
}

//export legate_image_attempt_close_v1
func legateImageAttemptClose(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleImageAttemptClose)
}

func main() {}
