# 用户标识与账户合同深度审阅

## Goal

对照主流身份库、标准协议和大型产品的一手资料，深度审阅 Identity 的 User 标识、公开资料、账号生命周期、
凭据与消费者合同；明确内部主体键、OIDC subject、公开用户 key 和人类可读 handle 的职责，修复真实缺陷并把
复杂性收敛在稳定、可演进的深模块接口后。

## Status

Finished

## Current

User 标识合同与消费者迁移已完成。内部 UUIDv4 只留在 Identity 数据边界；公开与跨服务引用统一使用
`usr_` + 128-bit base64url Public User Key；OIDC 显式分配 public/pairwise subject，token 以 `subject_kind`
区分 user、client、guest；handle、display name 和媒体引用各自独立。

旧 `/api/v1/profiles*` 已直接删除，新合同为 `/api/v1/users*`；资料媒体只存 Asset `mediaKey`，公开 URL 由各站
`/media/{mediaKey}?format=...&name=...` 组装。注册、管理员创建、角色与标识分配使用原子事务；删除用户不再公开；
公开 batch 有界且由 repository 批量查询。F1–F9 的实现结论和验收证据见
[实施与验收结果](references/implementation-and-acceptance.md)。

## Next

发布时先发布 Foundation 的 typed `subject_kind` 合同和 Identity Nuxt 包，再升级各产品锁定依赖；Gallery 当前安装的
`@yueli/identity-nuxt@0.1.0` 仍会请求已删除的 `/api/v1/profiles/{sub}`，仅因旧包把该补充资料请求降级而不阻断登录。
不得因此恢复旧接口。源码迁移和本地 CLI Playwright 已通过，发布与跨仓提交等待单独授权。

## Current execution

P0–P4 complete；生产源码、迁移、消费者、生成合同、全量测试、静态检查和 CLI Playwright 均已收口。

## Progress

- 2026-08-02：确认 Identity 工作树干净；新建独立 User 合同深审，避免重复已完成的认证安全与部署审阅。
- 2026-08-02：完成 Identity/Account/Nuxt 基线、OpenAPI、静态分析、漏洞扫描与跨仓 subject 消费审计。
- 2026-08-02：完成 55 项一手官方引用的市场调研和本地合同审阅，形成 ID 分层、User 深模块与迁移建议。
- 2026-08-02：获得生产重构授权；将稳定术语写入根 `CONTEXT.md`，并在本 Work 固化证据链接与实施决策。
- 2026-08-02：实现 Public User Key、handle 历史、public/pairwise subject、`subject_kind`、受控媒体引用、原子创建与
  `/api/v1/users*`，删除未发布的 `/api/v1/profiles*`。
- 2026-08-02：迁移 Asset、Blog、Commerce、Docs、Gallery、Nav、Resource、Shop 与 Workspace 本地夹具；修复 Gallery
  复用数据库中旧 UUID 管理员授权的确定性 devseed 对账。
- 2026-08-02：Identity/Account/Nuxt 与 9 个 Go 消费模块全量测试和 `go vet` 通过；Account 生产构建通过；CLI
  Playwright 2/2 通过，图片站保持运行供人工查看。

## References

- [稳定范围与问题定义](context.md)
- [执行计划](plan.md)
- [上一轮 Identity 深审](../2026-07-29-identity-deep-review/index.md)
- [主流实践调研](references/user-identity-market-research.md)
- [本地合同与消费者审计](references/current-contract-audit.md)
- [实施与验收结果](references/implementation-and-acceptance.md)
