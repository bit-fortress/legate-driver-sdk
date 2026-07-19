# 安全边界

[English](security.md)

Driver module 只能收到不含密钥的配置、endpoint ref 名称、credential slot 名称及其
`configured` 布尔值、精确协议 JSON 和选中的上游模型。

Driver 永远不会收到：

- Credential 值；
- 调用方 API Key、Authorization、Cookie 或转发链 Header；
- Workspace、ModelGroup、Endpoint 数据库实体；
- 候选列表、tier、attempt index 或 breaker 状态。

Driver 永远不执行网络 I/O。它只能返回受限 `RequestPlan`，其中的 `AuthPlan` 引用
credential slot，由 Core 注入真实凭据并执行 HTTP 请求。

Core 负责校验协议请求与响应、重复 JSON Key、资源预算、RequestPlan 限制、Header、
URL、状态码、SSE/NDJSON framing 和事件状态机。Core 还负责凭据、transport、超时、
路由、故障转移、熔断和调用分析。

Driver error、vendor code 和 field issue 必须有界、不包含密钥、URL、Body 或控制字符。
所有 Guest 输出在 Core 执行网络 I/O 或改变公开响应状态前都会被完整校验。

图片调用中的文件和大型上游响应 bytes 保存在 attempt-scoped Host spool。Guest 只收到
`BlobRef` 元数据，没有打开 blob 或获知本地路径的 API。Driver 可以在受限 BodyPlan 中
引用 blob，包括带 padding 的标准 Base64；Core 会在网络 I/O 或公开响应提交前校验
ownership、长度、摘要、展开长度、Header 和 JSON。Attempt Close 后全部 BlobRef 失效。
