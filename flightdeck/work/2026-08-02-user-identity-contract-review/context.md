# Context

## 范围

- 核心对象是 User 及其标识、公开资料、账号生命周期和跨服务消费合同。
- 审阅内部 UUID、OIDC `sub`、公开 profile ID、username/handle、外部身份 binding 和管理员 ID 的职责是否混杂。
- 同时检查注册、资料更新、邮箱、停用/删除、Guest 认领、会话与凭据接口是否泄漏实现或重复领域规则。
- OAuth/OIDC 密码学、MFA/Passkey 和部署只在与 User 合同直接相关时复核，不重复上一轮已完成深审。
- 先形成证据、影响面和目标合同，再修改生产实现；跨仓迁移必须和 Identity 合同一起验证。

## 重构前事实

- `identities.id` 使用 PostgreSQL `gen_random_uuid()`，是关系与领域主体的主键。
- 当前公开资料接口以同一个 ID 查询 `/api/v1/profiles/{id}`，响应字段也命名为 `id`。
- 管理员用户、角色、审计及多个跨服务合同直接传递 Identity ID。
- `user_profiles.username` 可空且唯一，当前没有独立的、明确规范化的公开 handle 生命周期。
- OIDC `sub` 必须稳定且不可重新分配；是否与内部主键相同、是否对不同 client 使用 pairwise subject，需要基于协议、
  隐私要求与现有消费者证据决定。

## 已验证证据

- [本地合同与消费者审计](references/current-contract-audit.md)：记录 Identity、Account、Nuxt 与产品服务的生产、存储、
  传递和公开使用点，并给出 F1–F9 缺陷、影响等级和迁移顺序。
- [规范与市场调研](references/user-identity-market-research.md)：汇总 OIDC、SCIM、UUID、主流身份库和大型网站的一手资料，
  共保留 55 项官方引用，作为 subject、public key、handle、生命周期和版本策略的复用依据。

后续工作先复读以上两份材料；只有合同前提、标准或外部产品行为发生实质变化时才重新调研。

## 已实施合同

- 内部主键继续使用 UUIDv4，但公开 API、跨服务文本所有权列、产品管理员配置和审计用户 actor 改用 Public User Key。
- Public User Key 格式固定为 `usr_` + 22 位 base64url；OIDC pairwise subject 使用独立的 `psu_` 命名空间。
- 新增 `GET /api/v1/users/{userKey}`、`GET /api/v1/users/by-handle/{handle}` 与有界批量
  `GET /api/v1/users?ids=...`；未发布的 `/api/v1/profiles*` 已删除且实测 404。
- handle 为小写 ASCII 3–30 位，保留词受控，历史值不重新分配；display name 继续允许 Unicode、重复与修改。
- 资料中的 avatar/cover 只保存 Asset `mediaKey`，社交链接由专用端点校验并限制数量；OIDC `picture` 指向公开媒体源。
- token 必须携带 `subject_kind=user|client|guest`。Foundation verifier fail closed；产品服务不再根据 `sub` 或
  `client_id` 的形状猜测主体种类。
- 注册与管理员创建在一个事务内完成 identity、public key、public OIDC subject、profile、credential 和初始 role；
  删除用户不会继续从公开 User API 返回。
- 详细缺陷闭环、消费者范围、测试命令与发布顺序见
  [实施与验收结果](references/implementation-and-acceptance.md)。

## 已确定的实施决策

- 保留 UUIDv4 作为 Internal User ID，不为字符串外观重写内部主键。
- 新增 `usr_` 前缀、128-bit 随机且 base64url 编码的 Public User Key，供公开 API、永久链接和跨服务公开引用使用。
- User Handle 与 Display Name 分离；第一版 handle 使用小写 ASCII、长度 3–30、保留词和不可重新分配的历史记录。
- 通用 Profile 更新不接受 avatar、cover 或社交链接 URL；媒体只能通过 Asset 工作流建立受控引用。
- 公开用户接口从 `/api/v1/users` 起步；未发布的 `/api/v1/profiles*` 直接移除，不保留兼容或 fallback。
- OIDC 标准端点不机械增加版本路径；subject 分配显式区分 public 与 pairwise 范围，身份键始终为 `(iss, sub)`。
- User 领域边界统一负责标识生成与解析、handle 规则、公开资料、生命周期和创建原子性；HTTP、OIDC 和存储适配器不重复规则。

## 审阅准则

- ID 首先按职责评估，不以字符串“好看”替代稳定性、不可重用、隐私和迁移成本。
- 内部主键可以保持实现友好；面向用户的 URL 与公开合同应隐藏不必要的存储实现。
- handle 是可读地址或展示标识时，不能替代不可变所有权键。
- 深模块用小接口隐藏规范化、保留词、重命名、别名、冲突和解析策略；消费者不重复这些规则。
- 未发布的新接口不保留废弃兼容层；已经被真实消费者持久化的主体合同必须先完成影响审计。

## 验收

- 调研只使用规范、官方文档、官方源码或一方接口文档，并逐项引用。
- ID/handle/profile/lifecycle 的生产者、存储者和消费者矩阵完整。
- 目标合同说明版本策略、迁移、缓存、隐私、枚举风险和删除/重命名语义。
- 适用 Go、Account、Nuxt、消费者测试以及 CLI Playwright 验收有明确证据。

## 发布边界

- 生产源码已完成，但本 Work 不执行包发布或跨仓提交。
- Gallery 当前锁定的 `@yueli/identity-nuxt@0.1.0` 仍包含旧 `/profiles` 补充查询；先发布新 Identity Nuxt 包、
  再升级产品依赖，不能恢复旧 API 兜底。
- Foundation typed `SubjectKind` 需先发布，再允许消费者从当前 raw verified claim 读取方式切到 typed 字段。
