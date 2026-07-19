# Driver Authoring

[简体中文](authoring.zh-CN.md)

## Mental model

A Text Driver translates one Legate inbound protocol into a vendor request and
translates the vendor result back into the same inbound protocol. It does not
act as an HTTP client or router.

Core owns endpoint selection, credentials, HTTP I/O, timeouts, cancellation,
capacity, retry and failover, breakers, upstream stream framing, output
validation, response commit, usage accounting, and analytics.

The Driver owns:

- endpoint configuration validation and frozen bound state;
- protocol-field handling and vendor request construction;
- buffered response and error conversion;
- stream event conversion and stream termination;
- usage extraction;
- an optional semantic outcome override for unusual vendor semantics.

## 1. Choose exact contracts

The initial Text contracts are:

```text
openai.chat_completions/2026-07-18
openai.responses/2026-07-18
anthropic.messages/2026-07-18
```

Start with one contract. Declare another only after both its buffered and SSE
paths are implemented and tested.

Declaring a contract means the Driver accepts every structurally valid request
in that contract. For every field, the Driver must deterministically preserve,
convert, ignore, or reject it. A semantic rejection is a protocol-native
`ClientResponse`; it is not an operational `DriverError`.

Contract IDs are exact. A new contract version is not implicitly supported.

## 2. Write the manifest

The manifest and typed handler registrations must contain the same exact set:

```json
{
  "id": "vendor-chat",
  "displayName": "Vendor Chat Driver",
  "version": "1",
  "kind": "text",
  "text": {
    "protocolContracts": ["openai.chat_completions/2026-07-18"]
  },
  "managementCapabilities": [],
  "configSchema": {"type": "object", "additionalProperties": false},
  "credentialSchema": {
    "slots": [{"name": "api_key", "required": true}]
  },
  "requestedCapabilities": [],
  "wireAbiVersion": "1"
}
```

Do not declare buffered/SSE modes or tools, reasoning, image input, or JSON
Schema feature flags. A protocol declaration already includes both output
lifecycles, and field behavior belongs to the handler.

WASM ABI v1 does not support management capabilities and does not grant
requested capabilities. Both arrays must be present and empty; a non-empty
value is rejected when the manifest is uploaded.

## 3. Implement Bind

Implement `TextBinder`. Bind receives:

- non-secret `config_json`;
- allowed endpoint reference names;
- credential slot names and `configured` booleans.

It never receives credential values, database entities, workspace identity, or
candidate routing information. Validate all required references and slots, then
return immutable bound state for later attempts.

## 4. Implement typed handlers

Implement the versioned protocol package's `Handler` and `Attempt`. Do not read
`protocol_contract` and write a manual switch. The SDK Dispatcher decodes the
request and selects the registered typed handler.

Go registration:

```go
dispatcher := driver.MustNewDispatcher(
    []string{chat.Contract},
    vendorBinder{},
    chat.Register(vendorChatHandler{}),
)
guest := driver.NewGuest(dispatcher)
```

Rust registration:

```rust
let dispatcher = sdk::Dispatcher::new(
    &[chat::CONTRACT],
    VendorBinder,
    vec![chat::register(VendorChatHandler)],
)
.expect("manifest and typed handlers must match");
let guest = sdk::Guest::new(dispatcher);
```

Dispatcher construction rejects missing, extra, duplicate, and unknown handler
registrations. It publishes the normalized protocol capability list at Bind.

Protocol decoders reject duplicate object keys at every depth and preserve
unknown fields in `ExtraFields`/`extra_fields`. Use the protocol package's
decoder and encoder instead of decoding the original request into an ad hoc
map.

## 5. Implement the attempt lifecycle

One attempt belongs to one real upstream candidate. The lifecycle is:

```text
Bind
  -> OpenAttempt
  -> RequestPlan
  -> TransformBuffered
  -> Close
```

or:

```text
Bind
  -> OpenAttempt
  -> immediate ClientResponse
  -> Close
```

or:

```text
Bind
  -> OpenAttempt
  -> RequestPlan
  -> OpenStream
  -> TransformStreamEvent repeated zero or more times
  -> FinishStream
  -> Close
```

`OpenAttempt` receives the typed request and metadata, invocation mode, selected
upstream model, and response ID. Replace the public model-group selector with
the selected upstream model when constructing the vendor request.

Return one stateful attempt plus exactly one output:

- `RequestPlan` for Core to execute; or
- protocol-native `ClientResponse` for an immediate result.

`Close` is called exactly once for success, failure, cancellation, timeout, or
failover. It must release all attempt-owned state.

## 6. Build a safe RequestPlan

A RequestPlan may use only endpoint and credential slot names declared by the
manifest and accepted at Bind. It contains a method, relative path, ordered
query values, headers, body, and an `AuthPlan` that references a credential
slot.

Never include:

- an absolute URL or arbitrary host;
- a credential value;
- caller authorization, cookies, or forwarding headers;
- retry, candidate, tier, or breaker instructions.

The Driver never performs network I/O.

## 7. Return protocol-native output

Buffered transforms return the versioned protocol package's `ClientResponse`.
Choose its typed success or error body according to the caller-facing response
you intend to produce.

Stream transforms return the same protocol package's typed `StreamEvent`
values. Core frames upstream SSE data fields and NDJSON records into normalized
stream-hook inputs, then validates downstream event order and terminal
semantics. Across stream event and finish results, `Outcome` and `Usage` must
each be reported at least once.

## 8. Outcome and failover

Normally omit Outcome. Core derives the default semantic class from the real
upstream status and owns all failover decisions.

Only override Outcome when the default classification does not express the real
semantics, for example an upstream HTTP 400 error that actually means the
configured upstream model does not exist:

```go
response := chat.Response(statusCode, headers, body).
    WithOutcome(driver.MappingError("vendor_model_not_found"))
```

Allowed classes are:

- `success`;
- `caller_error`;
- `endpoint_error`;
- `mapping_error`.

Outcome describes what happened. It never says whether to retry, whether a
response is final, or which candidate to use next.

Outcome and caller-facing status must agree: `success` requires a 2xx response;
the three error classes require a non-2xx response.

## 9. DriverError

Use `DriverError` only for operational failures. Protocol-level rejections are
protocol-native `ClientResponse` values. Core accepts only these codes from each
Text hook:

- Bind: `invalid_config`, `driver_internal`, `driver_resource_exhausted`.
- OpenAttempt: `unsupported_feature`, `unsupported_invocation_mode`,
  `protocol_request_invalid`, `invalid_invocation`, `driver_internal`,
  `driver_resource_exhausted`.
- TransformBuffered: `protocol_response_invalid`,
  `malformed_upstream_response`, `driver_internal`,
  `driver_resource_exhausted`.
- OpenStream: `malformed_upstream_response`, `driver_internal`,
  `driver_resource_exhausted`.
- TransformStreamEvent: `protocol_response_invalid`,
  `malformed_upstream_response`, `upstream_stream_truncated`,
  `driver_internal`, `driver_resource_exhausted`.
- FinishStream: `protocol_response_invalid`, `upstream_stream_truncated`,
  `driver_internal`, `driver_resource_exhausted`.
- Close: `invalid_attempt`, `invalid_attempt_state`, `driver_internal`,
  `driver_resource_exhausted`.

Returning a code outside the current hook's set is invalid Driver output. Error
messages, field issues, and vendor codes must be bounded and secret-safe.

## 10. Usage

Report final usage when the vendor provides authoritative input and output token
counts. Report partial usage only when at least one count is known. Otherwise
report unavailable usage. Do not invent token counts.

Every immediate or buffered `ClientResponse` requires Usage. On a successful SSE
path, stream event and finish results must collectively report Usage at least
once. Outcome may be omitted on `ClientResponse`, but successful SSE output must
also report Outcome at least once because there is no terminal HTTP response
body from which Core can derive it.

Only buffered, OpenStream, TransformStreamEvent, and FinishStream failures may
carry Usage, and failure Usage must be `partial` or `unavailable`. Bind,
OpenAttempt, and Close failures must not carry Usage.

## 11. Synchronous Image Drivers

Image Drivers declare either or both exact contracts:

```text
openai.images.generations/2026-07-19
openai.images.edits/2026-07-19
```

Use `ImageBinder`, `ImageDispatcher`, `ImageGuest`, and the versioned Generation
and Edit packages. An Image module exports bind plus only the three Image
attempt hooks; it does not export Text hooks. Image attempts are always
buffered and close exactly once.

Generation arrives as strict typed JSON. Edit arrives as ordered multipart
descriptors. Parts with filenames contain an opaque `BlobRef`, never file bytes
or a path. Preserve repeated image parts, mask, unknown parts, filenames,
Content-Type, and ordering unless the handler explicitly rejects or changes
them.

Build request and response bodies with the bounded BodyPlan builders:

```go
body := driver.NewCompositeBody(driver.MediaTypeJSON).
    AddInline([]byte(`{"image":"`)).
    AddBase64(imageRef).
    AddInline([]byte(`"}`))
```

Core streams Blob and standard padded Base64 sources. The Driver cannot open a
BlobRef. BodyPlan does not support nested multipart, URL-safe Base64, image
transforms, file paths, URLs, scripts, compression, or arbitrary Host calls.

Use the selected upstream model for candidate-local rules. For example, if the
current model supports at most four valid image parts, return
`unsupported_feature`; Core may try another candidate without penalizing the
breaker. Malformed or semantically invalid protocol requests must instead be
an immediate OpenAI Images error response. If every candidate rejects the
feature, Core produces the stable public 400 response.

Every immediate or transformed `ImageClientResponse` includes Usage. Use
unavailable Usage when the vendor has no authoritative accounting. A large
compatible upstream JSON response may be returned with its upstream BlobRef;
Core validates and spools the complete JSON before caller response commit.

Image hook error ownership is narrower than Text: Open allows
`unsupported_feature`, `protocol_request_invalid`, `invalid_invocation`,
`driver_internal`, and `driver_resource_exhausted`; Transform allows
`protocol_response_invalid`, `malformed_upstream_response`, `driver_internal`,
and `driver_resource_exhausted`; Close allows attempt-state and operational
errors. Open and Close failures never carry Usage.

## 12. Completion checklist

Before publishing a Driver:

- Manifest contracts exactly match registered typed handlers.
- Every declared Text contract implements buffered and SSE paths; every Image
  contract implements its synchronous lifecycle.
- Unknown fields have an explicit preserve, convert, ignore, or reject policy.
- Success, vendor error, malformed response, truncated stream, and cancellation
  paths are deterministic and bounded.
- Direct and transformed responses use the original inbound contract.
- Immediate and buffered responses report Usage; successful SSE output reports
  both Usage and Outcome across its event and finish results.
- Each DriverError uses a code allowed by the hook that returns it.
- Every opened attempt can be closed exactly once.
- No secret, absolute URL, or routing instruction appears in Driver output.
- `make test` passes.
- All Go/Rust Text and Image example modules compile, import nothing, export one memory, and have
  only the current ABI v1 hooks.

Use the Go and Rust Chat-compatible examples as the starting templates:

- `go/examples/openai-chat-compatible`
- `rust/examples/openai-chat-compatible`
- `go/examples/openai-image-compatible`
- `rust/examples/openai-image-compatible`
