package main

import (
	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

var dispatcher = sdk.MustNewDispatcher(
	[]string{chat.Contract},
	openAIChatBinder{},
	chat.Register(openAIChatHandler{}),
)
var guest = sdk.NewGuest(dispatcher)

//export legate_alloc_v1
func legateAlloc(size uint32) uint32 {
	return sdk.Alloc(size)
}

//export legate_free_v1
func legateFree(pointer uint32, size uint32) {
	sdk.Free(pointer, size)
}

//export legate_bind_v1
func legateBind(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleBind)
}

//export legate_text_attempt_open_v1
func legateTextAttemptOpen(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextAttemptOpen)
}

//export legate_text_transform_buffered_response_v1
func legateTextTransformBufferedResponse(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextTransformBufferedResponse)
}

//export legate_text_sse_open_v1
func legateTextSSEOpen(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextSSEOpen)
}

//export legate_text_sse_transform_event_v1
func legateTextSSETransformEvent(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextSSETransformEvent)
}

//export legate_text_sse_finish_v1
func legateTextSSEFinish(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextSSEFinish)
}

//export legate_text_attempt_close_v1
func legateTextAttemptClose(pointer uint32, size uint32) uint64 {
	return sdk.Invoke(pointer, size, guest.HandleTextAttemptClose)
}

func main() {}
