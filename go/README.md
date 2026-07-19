# Go SDK

[简体中文](README.zh-CN.md)

Implement `driver.TextBinder`, then implement the `Handler` and `Attempt`
interfaces from each versioned protocol package you declare. Register those
typed handlers with `driver.NewDispatcher`; construction fails when the
manifest contract list and registrations differ. The dispatcher decodes and
routes exact contracts, so Driver code does not switch on contract IDs.

`OpenAttempt` constructs the RequestPlan using the typed input's selected
upstream model; there is no separate Prepare hook. Each protocol `Attempt`
interface requires both buffered and SSE methods. Every immediate or buffered
ClientResponse must include Usage; successful SSE output reports both Outcome
and Usage across event and finish results.

Production guest modules use TinyGo 0.39.0 with target `wasm-unknown`. The
[`openai-chat-compatible`](examples/openai-chat-compatible) example supports
both buffered and SSE Chat Completions calls.

For synchronous images, implement `driver.ImageBinder` and the typed handlers
from `protocol/openai/images/{generations,edits}/v20260719`, register them with
`driver.NewImageDispatcher`, and export only the Image hook set. Use `BlobRef`
and `BodyPlan` builders for file forwarding or streaming Base64; the SDK never
exposes blob bytes. See [`openai-image-compatible`](examples/openai-image-compatible).
