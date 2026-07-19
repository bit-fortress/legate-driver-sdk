# Driver 开发指南

[English](authoring.md)

## 基本认知

Text Driver 把一种 Legate 入站协议转换为供应商请求，再把供应商结果转换回同一种
入站协议。Driver 不是 HTTP 客户端，也不是路由器。

Core 负责接入点选择、凭据、HTTP I/O、超时、取消、容量、重试与故障转移、熔断、
上游流 framing、输出校验、响应提交、用量计量和调用分析。

Driver 负责：

- 校验接入点配置并生成冻结的 bound state；
- 处理协议字段并构造供应商请求；
- 转换 buffered 成功响应与错误响应；
- 转换流事件并处理流终止；
- 提取 Usage；
- 在供应商语义特殊时可选覆盖标准 Outcome。

## 1. 选择精确 Contract

首批 Text contract：

```text
openai.chat_completions/2026-07-18
openai.responses/2026-07-18
anthropic.messages/2026-07-18
```

第一个 Driver 建议先实现一个 contract。只有 buffered 和 SSE 都实现并测试完成后，
才能继续声明其他 contract。

声明一个 contract，意味着 Driver 接受该 contract 下所有结构合法的请求。对于每个
字段，Driver 必须确定地选择保留、转换、忽略或拒绝。语义拒绝应该返回协议原生
`ClientResponse`，而不是 operational `DriverError`。

Contract ID 必须精确匹配。发布新 contract 版本后，老 Driver 不会自动获得兼容声明。

## 2. 编写 Manifest

Manifest 与 typed handler 注册必须包含完全相同的 contract 集合：

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

不要再声明 buffered/SSE mode，也不要声明 tools、reasoning、图片输入或 JSON Schema
等 feature。声明协议已经包含两种输出生命周期，具体字段行为属于 handler。

WASM ABI v1 不支持 management capability，也不会授予 requested capability。这两个
数组必须存在且为空；上传 Manifest 时，任何非空值都会被拒绝。

## 3. 实现 Bind

实现 `TextBinder`。Bind 只会收到：

- 不含密钥的 `config_json`；
- 允许使用的 endpoint ref 名称；
- credential slot 名称和 `configured` 布尔值。

Bind 不会收到凭据值、数据库实体、Workspace 身份或候选路由信息。校验所有必要的
ref 和 slot 后，返回供后续 attempt 使用的不可变 bound state。

## 4. 实现 Typed Handler

实现版本化协议包提供的 `Handler` 和 `Attempt`。不要读取 `protocol_contract` 后手写
`switch`。SDK Dispatcher 会解码请求并选择已经注册的 typed handler。

Go 注册：

```go
dispatcher := driver.MustNewDispatcher(
    []string{chat.Contract},
    vendorBinder{},
    chat.Register(vendorChatHandler{}),
)
guest := driver.NewGuest(dispatcher)
```

Rust 注册：

```rust
let dispatcher = sdk::Dispatcher::new(
    &[chat::CONTRACT],
    VendorBinder,
    vec![chat::register(VendorChatHandler)],
)
.expect("manifest and typed handlers must match");
let guest = sdk::Guest::new(dispatcher);
```

Dispatcher 构造时会拒绝缺失、多余、重复或未知的 handler 注册，并在 Bind 时发布规范化
后的协议能力列表。

协议 decoder 会拒绝任意层级的重复对象 Key，并把未知字段保存在
`ExtraFields`/`extra_fields`。应该使用协议包提供的 decoder 和 encoder，不要把原始
请求随意解码为临时 map。

## 5. 实现 Attempt 生命周期

一个 attempt 只属于一个真实上游候选。生命周期为：

```text
Bind
  -> OpenAttempt
  -> RequestPlan
  -> TransformBuffered
  -> Close
```

或者：

```text
Bind
  -> OpenAttempt
  -> 立即返回 ClientResponse
  -> Close
```

或者：

```text
Bind
  -> OpenAttempt
  -> RequestPlan
  -> OpenStream
  -> TransformStreamEvent 重复零到多次
  -> FinishStream
  -> Close
```

`OpenAttempt` 接收 typed request、typed metadata、调用模式、选中的上游模型和响应 ID。
构造供应商请求时，需要用 selected upstream model 替换公开的模型组选择器。

返回一个有状态 attempt，并且以下输出必须二选一：

- 交给 Core 执行的 `RequestPlan`；
- 立即返回的协议原生 `ClientResponse`。

无论成功、失败、取消、超时还是故障转移，`Close` 都会恰好调用一次。它必须释放
attempt 持有的全部状态。

## 6. 构造安全的 RequestPlan

`RequestPlan` 只能引用 Manifest 声明且 Bind 已接受的 endpoint 和 credential slot。
它可以包含 method、相对路径、有序 Query、Headers、Body，以及引用 credential slot
的 `AuthPlan`。

禁止包含：

- 绝对 URL 或任意 Host；
- 凭据值；
- 调用方 Authorization、Cookie 或转发链 Header；
- retry、候选、tier 或 breaker 指令。

Driver 永远不自行执行网络 I/O。

## 7. 返回协议原生输出

Buffered 转换返回版本化协议包的 `ClientResponse`。根据希望对调用方表达的响应，选择
该协议的 typed success body 或 error body。

流式转换返回同一协议包的 typed `StreamEvent`。Core 会把上游 SSE data 字段或 NDJSON
record 解帧成归一化的 stream hook 输入，再校验下游事件顺序和终止语义。在 stream event
与 finish 的合并结果中，`Outcome` 和 `Usage` 都必须至少报告一次。

## 8. Outcome 与故障转移

普通响应不要填写 Outcome。Core 会根据真实上游状态推导默认语义，并独占所有故障
转移决策。

只有默认分类无法表达真实语义时才覆盖 Outcome，例如上游返回 HTTP 400，但实际含义是
配置的上游模型不存在：

```go
response := chat.Response(statusCode, headers, body).
    WithOutcome(driver.MappingError("vendor_model_not_found"))
```

允许的分类：

- `success`；
- `caller_error`；
- `endpoint_error`；
- `mapping_error`。

Outcome 只描述“发生了什么”，不描述“是否重试”“响应是否 final”或“下一个候选是谁”。

Outcome 必须与调用方状态码一致：`success` 要求 2xx 响应，另外三种错误分类要求非 2xx
响应。

## 9. DriverError

只有 operational failure 才使用 `DriverError`。协议层拒绝应该返回协议原生
`ClientResponse`。Core 对每个 Text hook 只接受以下错误码：

- Bind：`invalid_config`、`driver_internal`、`driver_resource_exhausted`。
- OpenAttempt：`unsupported_feature`、`unsupported_invocation_mode`、
  `protocol_request_invalid`、`invalid_invocation`、`driver_internal`、
  `driver_resource_exhausted`。
- TransformBuffered：`protocol_response_invalid`、
  `malformed_upstream_response`、`driver_internal`、
  `driver_resource_exhausted`。
- OpenStream：`malformed_upstream_response`、`driver_internal`、
  `driver_resource_exhausted`。
- TransformStreamEvent：`protocol_response_invalid`、
  `malformed_upstream_response`、`upstream_stream_truncated`、
  `driver_internal`、`driver_resource_exhausted`。
- FinishStream：`protocol_response_invalid`、`upstream_stream_truncated`、
  `driver_internal`、`driver_resource_exhausted`。
- Close：`invalid_attempt`、`invalid_attempt_state`、`driver_internal`、
  `driver_resource_exhausted`。

返回当前 hook 集合之外的错误码会被视为非法 Driver 输出。错误 message、field issue 和
vendor code 必须有界且不包含密钥。

## 10. Usage

供应商提供可信输入和输出 token 数时报告 final usage；只知道部分计数时才能报告
partial usage；否则报告 unavailable usage。不要自行编造 token 数。

立即返回或 buffered 的每个 `ClientResponse` 都必须包含 Usage。成功 SSE 路径的 stream
event 与 finish 结果合并后必须至少报告一次 Usage。`ClientResponse` 可以省略 Outcome；
但成功 SSE 输出还必须至少报告一次 Outcome，因为 Core 无法从终止 HTTP response body
推导它。

只有 buffered、OpenStream、TransformStreamEvent 和 FinishStream 的失败可以携带
Usage，并且失败 Usage 只能是 `partial` 或 `unavailable`。Bind、OpenAttempt 和 Close
失败不能携带 Usage。

## 11. 同步 Image Driver

图片 Driver 声明以下一个或两个精确 contract：

```text
openai.images.generations/2026-07-19
openai.images.edits/2026-07-19
```

使用 `ImageBinder`、`ImageDispatcher`、`ImageGuest` 和版本化 Generation/Edit 包。
Image module 只导出 bind 和三个 Image attempt hooks，不导出 Text hooks。图片 attempt
始终为 buffered，并且恰好 Close 一次。

Generation 以 strict typed JSON 进入。Edit 以保持顺序的 multipart descriptors 进入。
带 filename 的 part 只包含不透明 `BlobRef`，不包含文件 bytes 或路径。除非 handler
明确拒绝或转换，否则必须保留重复 image、mask、未知 part、filename、Content-Type 和顺序。

使用受限 BodyPlan builders 构造请求和响应：

```go
body := driver.NewCompositeBody(driver.MediaTypeJSON).
    AddInline([]byte(`{"image":"`)).
    AddBase64(imageRef).
    AddInline([]byte(`"}`))
```

Core 会流式读取 Blob，并执行带 padding 的标准 Base64。Driver 不能打开 BlobRef。
BodyPlan 不支持嵌套 multipart、URL-safe Base64、图片 transform、文件路径、URL、脚本、
压缩或任意 Host 调用。

可以用 selected upstream model 判断当前候选的局部约束。例如当前模型最多支持四张合法
输入图时返回 `unsupported_feature`；Core 可以尝试其他候选，并且不会惩罚 breaker。
请求本身 malformed 或协议语义无效时，必须立即返回 OpenAI Images error response，不能
伪装成候选能力错误。如果所有候选都拒绝该 feature，Core 统一生成稳定的公开 400 响应。

每个立即或转换后的 `ImageClientResponse` 都必须包含 Usage。供应商没有权威计量时使用
unavailable。大型兼容上游 JSON 可以直接引用 upstream BlobRef；Core 会在公开响应提交前
完整校验并 spool JSON。

图片 hook 的错误码所有权比文字更窄：Open 只允许 `unsupported_feature`、
`protocol_request_invalid`、`invalid_invocation`、`driver_internal` 和
`driver_resource_exhausted`；Transform 只允许 `protocol_response_invalid`、
`malformed_upstream_response`、`driver_internal` 和 `driver_resource_exhausted`；Close
只允许 attempt 状态和 operational errors。Open 与 Close failure 不能携带 Usage。

## 12. 完成检查

发布 Driver 前确认：

- Manifest contract 与注册的 typed handler 完全一致。
- 每个 Text contract 都实现 buffered 和 SSE；每个 Image contract 都实现同步生命周期。
- 未知字段有明确的保留、转换、忽略或拒绝策略。
- 成功、供应商错误、损坏响应、流截断和取消路径都有界且行为确定。
- 立即响应和转换后的响应仍属于原入站 contract。
- 立即响应和 buffered 响应包含 Usage；成功 SSE 输出在 event 与 finish 结果中合计报告
  Usage 和 Outcome。
- 每个 DriverError 都使用其返回 hook 允许的错误码。
- 每个已打开 attempt 都能恰好关闭一次。
- Driver 输出不包含密钥、绝对 URL 或路由指令。
- `make test` 通过。
- Go/Rust 的 Text 与 Image 示例 WASM 都能构建，零 imports、只导出一个 memory，并且只有当前 ABI v1
  hooks。

从以下示例开始修改：

- `go/examples/openai-chat-compatible`
- `rust/examples/openai-chat-compatible`
- `go/examples/openai-image-compatible`
- `rust/examples/openai-image-compatible`
