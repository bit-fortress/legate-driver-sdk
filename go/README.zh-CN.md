# Go SDK

[English](README.md)

实现 `driver.TextBinder`，并为 Manifest 声明的每个 contract 实现对应版本化协议包的
`Handler` 和 `Attempt` 接口。使用 `driver.NewDispatcher` 注册 typed handler；如果
Manifest contract 集合与注册集合不一致，构造会失败。

Driver 代码不需要读取 contract ID 或手写分发。`OpenAttempt` 使用 typed input 中的
selected upstream model 构造 `RequestPlan`，不存在单独的 Prepare hook。每个协议的
`Attempt` 接口同时要求 buffered 和 SSE 方法。立即返回或 buffered 的每个
`ClientResponse` 都必须包含 Usage；成功 SSE 输出在 event 与 finish 结果中合计报告
Outcome 和 Usage。

生产 Guest 使用 TinyGo 0.39.0 和 `wasm-unknown` target。示例：

- [`openai-chat-compatible`](examples/openai-chat-compatible)

该示例同时覆盖 Chat Completions 的 buffered 和 SSE 调用。

同步图片 Driver 实现 `driver.ImageBinder`，以及
`protocol/openai/images/{generations,edits}/v20260719` 的 typed handlers，再通过
`driver.NewImageDispatcher` 注册，并且只导出 Image hooks。文件转发或流式 Base64 使用
`BlobRef` 和 `BodyPlan` builders；SDK 不提供读取 blob bytes 的能力。参考：

- [`openai-image-compatible`](examples/openai-image-compatible)
