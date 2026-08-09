# ADR 0001：统一为 8 位 Public User Key

- 状态：Accepted
- 日期：2026-08-09

## 背景

Identity 需要同时满足数据库关系、跨服务引用、OIDC、公开资料地址和人工识读。为这些场景分别维护长 Public User Key 与短号，会产生重复唯一约束、双路由和消费者选型歧义。产品尚未发布，所有开发数据可由 seed 重建，因此不保留双轨兼容。

## 决策

- 内部 User 主键使用 Foundation Identifier 生成的 UUIDv7，只留在 Identity 数据边界。
- 唯一的 Public User Key 使用 Foundation Identifier `compact-url-v1`：精确 8 位 Bitcoin Base58。
- Identity 通过数据库唯一约束原子占用候选，碰撞时使用 `identifier.Allocate` 重试。
- Public User Key 一经分配不可修改、不可回收，同时承担 API `userKey`、OIDC `user_key`、跨服务用户引用、界面用户号和 `/u/{userKey}` 永久地址。
- `/@handle` 继续作为可修改、可读的首选资料地址；Handle 不承担授权或外键职责。
- 不保留 `shortId` 字段、`usr_` 前缀格式、短号查询端点或迁移兼容代码。

## 边界

Public User Key 可公开、可记录，不因“随机”而成为凭证。邀请、重置、会话、API Token 等 possession-based secret 必须由各自安全模块生成，不能使用 Public User Key Profile。

## 被否决的方案

- 数据库自增 ID 会暴露顺序和规模，并把存储实现泄漏到公开合同。
- Sqids/Hashids 只是序号编码，不提供随机公开标识的安全或演进收益。
- 长机器键加独立短号会重复身份语义，迫使每个站点决定保存哪一个。
- 只使用 Handle 无法提供不可变永久地址。
