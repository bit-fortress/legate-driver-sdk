# Legate Driver SDK

[English](README.md)

Legate 协议原生 Text Driver 与同步 Image Driver ABI v1 的 Go/TinyGo 与 Rust Guest SDK。
权威 wire contract 是 [`abi/v1/driver.proto`](abi/v1/driver.proto)。

Driver 声明它支持的精确入站协议 contract，并把每个请求转换为上游
`RequestPlan`。Driver 再把上游 buffered 响应或由 Core 解帧的流事件转换回同一种入站协议。

Core 当前负责 SSE 与逐行 JSON framing（包括 Ollama 使用 `application/json` 的流式变体），
并负责接入点选择、凭据、HTTP I/O、超时、重试与故障转移、熔断、校验、响应提交和
调用分析；Driver 不承担这些职责。

首批 contract：

- `openai.chat_completions/2026-07-18`
- `openai.responses/2026-07-18`
- `anthropic.messages/2026-07-18`
- `openai.images.generations/2026-07-19`
- `openai.images.edits/2026-07-19`

`TextAttemptOpen` 接收协议请求、协议 metadata、选中的上游模型、调用模式和响应标识，
返回 `RequestPlan` 或立即返回协议原生 `ClientResponse`。之后由 buffered 或 SSE hooks
转换上游结果。每条路径最终都恰好调用一次 `TextAttemptClose`。

Image module 使用 `OpenImageAttempt -> TransformBufferedResponse -> Close`，一期只支持
同步 JSON Generation 和 multipart Edit。图片二进制始终由 Host 持有，Driver 只能看到
不透明的 `BlobRef`，并使用受限 `BodyPlan` 组合请求或响应。`Base64(blob)` 由 Core
流式执行，不会把图片复制进 WASM heap。

从 [`docs/authoring.zh-CN.md`](docs/authoring.zh-CN.md) 开始阅读。

运行：

```bash
make test
make verify
```
