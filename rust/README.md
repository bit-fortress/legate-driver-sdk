# Rust SDK

[简体中文](README.zh-CN.md)

The `legate-driver-sdk` crate uses `prost` messages matching
`abi/v1/driver.proto`. `Guest<D>` owns one active attempt and its opaque handle.
`OpenAttempt` returns either a RequestPlan or immediate ClientResponse. Every
immediate or buffered ClientResponse must include Usage; successful SSE output
reports both Outcome and Usage across event and finish results.

Implement `TextBinder` plus the versioned protocol package's typed `Handler`
and `Attempt` traits. Register handlers with `Dispatcher::new`; construction
fails when manifest declarations and registered exact contracts differ. Driver
code never switches on contract IDs.

Build and test with Rust 1.97.0 and the `wasm32-unknown-unknown` target:

```bash
../scripts/run-rust-cargo.sh test --workspace
../scripts/run-rust-cargo.sh clippy --workspace --all-targets -- -D warnings
../scripts/run-rust-cargo.sh build --release --target wasm32-unknown-unknown -p openai-chat-compatible
```

For synchronous images, implement `ImageBinder` and the versioned Generation
and Edit handlers, then register them with `ImageDispatcher`. `ImageGuest`
exports only the Image hooks. Blob bytes stay Host-owned; compose bodies with
the BlobRef/BodyPlan helpers. The `openai-image-compatible` example implements
both image contracts.
