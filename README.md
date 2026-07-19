# Legate Driver SDK

[简体中文](README.zh-CN.md)

Go/TinyGo and Rust guest SDKs for Legate's protocol-native Text and synchronous
Image Driver ABI v1.
The authoritative wire contract is [`abi/v1/driver.proto`](abi/v1/driver.proto).

Drivers declare exact inbound protocol contracts and translate each request to
an upstream RequestPlan. They transform upstream buffered responses or
Core-framed stream events back to the same inbound protocol. Core currently
frames SSE and newline-delimited JSON upstream responses (including Ollama's
`application/json` streaming variant) and owns endpoint selection, credentials,
HTTP I/O, timeouts, retry and failover, breakers, validation, response commit,
and analytics.

The initial contracts are:

- `openai.chat_completions/2026-07-18`
- `openai.responses/2026-07-18`
- `anthropic.messages/2026-07-18`
- `openai.images.generations/2026-07-19`
- `openai.images.edits/2026-07-19`

`TextAttemptOpen` receives the protocol request, metadata, selected upstream
model, mode, and response identity. It returns a RequestPlan or an immediate
protocol-native ClientResponse. Buffered and SSE hooks then transform upstream
results. Every path ends with `TextAttemptClose` exactly once.

Image modules use `OpenImageAttempt -> TransformBufferedResponse -> Close` and
support synchronous JSON Generation and multipart Edit. Image bytes remain
Host-owned behind opaque `BlobRef` values. Drivers compose request and response
bodies with bounded `BodyPlan` builders; `Base64(blob)` is streamed by Core.

Start with [`docs/authoring.md`](docs/authoring.md). Chinese documentation is
available at [`docs/authoring.zh-CN.md`](docs/authoring.zh-CN.md).

Run:

```bash
make test
make verify
```
