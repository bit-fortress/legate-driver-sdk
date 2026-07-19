# Compatibility

[简体中文](compatibility.zh-CN.md)

| SDK | Protocol contracts | Wire ABI | Toolchain |
| --- | --- | --- | --- |
| Go `v0.x` | Explicit manifest set | Driver ABI v1 | Go 1.25.0, TinyGo 0.39.0 |
| Rust `0.x` | Explicit manifest set | Driver ABI v1 | Rust 1.97.0 |

Contract IDs are exact. There are no version ranges or automatic compatibility
claims. A Driver supports only contracts listed in its manifest and Bind
capabilities. The discarded pre-release ABI has no loader.

The active image contracts are `openai.images.generations/2026-07-19` and
`openai.images.edits/2026-07-19`. A future contract never changes an existing
image Driver's declarations. Go and Rust expose equivalent Image Dispatcher,
Attempt, BlobRef, BodyPlan, Generation, and Edit behavior.
