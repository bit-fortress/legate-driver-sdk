# Driver ABI v1

[English](README.md)

协议原生 ABI 使用 `driver.proto` 中的 protobuf messages。每个 module 都导出 alloc、
free、bind，并且只能选择一组 kind-specific hooks。

Text hooks：

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

Image hooks：

```text
legate_image_attempt_open_v1(i32, i32) -> i64
legate_image_transform_buffered_response_v1(i32, i32) -> i64
legate_image_attempt_close_v1(i32, i32) -> i64
```

Module 恰好导出一个 memory，不导入任何函数或 memory。可以额外导出 reactor 初始化
函数 `_initialize`。

`TextAttemptOpenSuccess.output` 是包含 `RequestPlan` 或 `ClientResponse` 的 oneof。
每个 `ClientResponse` 都包含 Usage；成功 SSE 输出在 event 与 finish 结果中合计报告
Outcome 和 Usage。Host 独占网络访问，并对每个已打开 attempt 恰好调用一次
`AttemptClose`。

图片文件 bytes 不进入 protobuf；`BlobRef` 只包含 attempt-local ID、长度和 SHA-256。
`BodyPlan` 只支持 Inline、Blob、标准 Base64、Composite 和非嵌套 Multipart。每个
`ImageClientResponse` 都必须包含 Usage，并在公开响应提交前完成全部校验。
