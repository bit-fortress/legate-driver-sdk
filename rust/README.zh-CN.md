# Rust SDK

[English](README.md)

`legate-driver-sdk` crate 使用与 `abi/v1/driver.proto` 一致的 `prost` messages。
`Guest<D>` 持有一个活动 attempt 及其 opaque handle。`OpenAttempt` 返回 `RequestPlan`
或立即返回 `ClientResponse`。立即返回或 buffered 的每个 `ClientResponse` 都必须包含
Usage；成功 SSE 输出在 event 与 finish 结果中合计报告 Outcome 和 Usage。

实现 `TextBinder`，以及对应版本化协议包的 typed `Handler` 和 `Attempt` traits。通过
`Dispatcher::new` 注册 handler；Manifest 声明与精确注册集合不一致时构造失败。
Driver 代码不需要读取 contract ID 或手写分发。

使用 Rust 1.97.0 和 `wasm32-unknown-unknown` target 构建与测试：

```bash
../scripts/run-rust-cargo.sh test --workspace
../scripts/run-rust-cargo.sh clippy --workspace --all-targets -- -D warnings
../scripts/run-rust-cargo.sh build --release --target wasm32-unknown-unknown -p openai-chat-compatible
```

同步图片 Driver 实现 `ImageBinder` 和版本化 Generation/Edit handlers，再注册到
`ImageDispatcher`。`ImageGuest` 只导出 Image hooks。Blob bytes 始终由 Host 持有，
Driver 使用 BlobRef/BodyPlan helpers 组合 body。`openai-image-compatible` 示例同时
实现两个图片 contract。
