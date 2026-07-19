# Driver ABI v1

[简体中文](README.zh-CN.md)

The protocol-native ABI uses protobuf messages from `driver.proto`. Every
module exports alloc, free, bind, and exactly one kind-specific hook set.

Text hooks:

```text
legate_alloc_v1(i32) -> i32
legate_free_v1(i32, i32)
legate_bind_v1(i32, i32) -> i64
legate_text_attempt_open_v1(i32, i32) -> i64
legate_text_transform_buffered_response_v1(i32, i32) -> i64
legate_text_sse_open_v1(i32, i32) -> i64
legate_text_sse_transform_event_v1(i32, i32) -> i64
legate_text_sse_finish_v1(i32, i32) -> i64
legate_text_attempt_close_v1(i32, i32) -> i64
```

Image hooks:

```text
legate_image_attempt_open_v1(i32, i32) -> i64
legate_image_transform_buffered_response_v1(i32, i32) -> i64
legate_image_attempt_close_v1(i32, i32) -> i64
```

The module exports exactly one memory and may additionally export the reactor
initializer `_initialize`. It imports no functions or memories.

`TextAttemptOpenSuccess.output` is a oneof containing RequestPlan or
ClientResponse. Every ClientResponse includes Usage. Successful SSE output
reports both Outcome and Usage across event and finish results. The Host owns
network access and calls AttemptClose exactly once.

Image file bytes never cross the protobuf boundary. `BlobRef` contains only an
attempt-local ID, length, and SHA-256 digest. `BodyPlan` supports Inline, Blob,
standard Base64, Composite, and non-nested Multipart sources. Every
ImageClientResponse includes Usage and is validated before response commit.
