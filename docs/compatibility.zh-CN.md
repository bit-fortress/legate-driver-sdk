# 兼容性

[English](compatibility.md)

| SDK | 协议 Contract | Wire ABI | 工具链 |
| --- | --- | --- | --- |
| Go `v0.x` | Manifest 显式声明的集合 | Driver ABI v1 | Go 1.25.0、TinyGo 0.39.0 |
| Rust `0.x` | Manifest 显式声明的集合 | Driver ABI v1 | Rust 1.97.0 |

Contract ID 必须精确匹配，不支持版本范围，也不自动推断兼容性。Driver 只支持 Manifest
声明并注册 typed handler 的 contract。

协议新增可选字段时，老 SDK 会把字段保存在 `ExtraFields`/`extra_fields`；Driver 仍需
确定地选择保留、转换、忽略或拒绝。协议出现破坏性变化时会发布新的 contract ID，老
Driver 不会自动支持。

重构前的预发布 ABI 已作废，不存在旧 ABI loader、兼容 export 或双运行时。

当前图片 contract 是 `openai.images.generations/2026-07-19` 和
`openai.images.edits/2026-07-19`。未来新增 contract 不会改变老图片 Driver 的声明。
Go 与 Rust 对 Image Dispatcher、Attempt、BlobRef、BodyPlan、Generation 和 Edit 提供
等价行为。
