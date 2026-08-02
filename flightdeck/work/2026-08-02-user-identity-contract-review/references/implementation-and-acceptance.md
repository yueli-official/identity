# User 标识合同实施与验收结果

## 结论

2026-08-02 已完成 User 标识、公开资料、OIDC subject 与消费者合同重构。内部 UUIDv4 保留在 Identity 数据边界；
公开和跨服务稳定引用统一使用 `usr_` + 128-bit base64url Public User Key。未发布的旧 Profile API 已删除，
没有兼容层或 fallback。

## 最终合同

| 职责 | 合同 |
| --- | --- |
| 内部关系与事务 | UUIDv4 Internal User ID，只在 Identity 内使用 |
| 公开与跨服务引用 | `usr_` + 22 位 base64url Public User Key，不可变、不可复用 |
| OIDC 第一方 subject | 显式 public assignment，值可为 Public User Key |
| OIDC 隐私 subject | `psu_` + 22 位 base64url，按 sector 稳定分配 |
| 人类可读地址 | 小写 ASCII handle，3–30 位，保留词受控，历史值不重新分配 |
| 展示 | 可重复、可修改的 Unicode display name |
| 媒体 | 资料只存 Asset `mediaKey`；站点通过 `/media/{mediaKey}` 交付 |
| 主体类型 | token 强制 `subject_kind=user|client|guest`，消费者 fail closed |

公开读取接口为：

- `GET /api/v1/users/{userKey}`
- `GET /api/v1/users/by-handle/{handle}`
- `GET /api/v1/users?ids={userKey,...}`

`/api/v1/profiles*` 已删除。OIDC 标准端点不增加业务版本路径。

## 审计缺陷闭环

- F1：通用 Profile 不再接受任意 avatar/cover URL；社交链接使用专用端点、URL 校验和 8 条上限。
- F2：handle 规则集中在 User 深模块；空值、规范化、保留词、冲突和历史不可复用有统一测试。
- F3：公开 User 查询保留基础设施错误，有界 batch 由 repository 批量读取，不再无上限 N+1。
- F4：删除用户从公开 User 读取中隐藏；管理、隐私和 session 路径统一使用明确生命周期语义。
- F5：注册和管理员创建原子写入 identity、public key、public subject、profile、credential 与初始 role。
- F6：Memory/PostgreSQL repository 补齐 public key、OIDC subject、handle history、batch 与原子创建合同。
- F7：内部 UUID 与公开 key、OIDC subject、handle、display name 完成职责拆分。
- F8：Foundation 和所有消费者显式识别 `subject_kind`，不再从 `sub`/`client_id` 字符串猜测。
- F9：新 users、admin、session、PAT、MFA、Passkey 和 recovery wire contract 不再暴露内部 UUID；错误目录同步。

## 消费者迁移

- Identity Account 与 Identity Nuxt 使用 `userKey`、handle 和 media reference；Account 增加同源 `/media` 代理。
- Blog 改用 `/api/v1/users*` 解析作者，并使用 `mediaKey`；旧 `/profiles` 调用从源码删除。
- Asset、Blog、Commerce、Docs、Gallery、Nav、Resource、Shop 的授权与审计只接受声明的 user/client/guest 类型。
- 产品本地管理员、owner 和演示内容夹具改用 Public User Key；内部 UUID 只用于 Identity devseed 的内部主键。
- Gallery devseed 会把复用数据库中的确定性旧 UUID bootstrap grant 对账为 Public User Key，避免本地授权快照残留。

## 验收证据

以下命令在 2026-08-02 通过：

- Identity：`go test ./... -count=1 -timeout 240s`、`go run ./cmd/errorcatalog --check`、`go vet ./...`、核心命令 build。
- Account：`pnpm test`（53）、`pnpm typecheck`、`pnpm build`。
- Identity Nuxt：`pnpm test`（30）、`pnpm pack --dry-run`。
- Foundation Go、Asset、Blog API、Commerce、Docs API、Gallery API、Nav API、Resource API、Shop API：各自
  `go test ./... -count=1 -timeout 240s` 与 `go vet ./...`；Asset、Gallery 另完成命令 build。
- Gallery 完整本地组合健康：Identity `8081`、Account `3010`、Asset `8082`、Gallery API `8091`、Gallery Web `3007`。
- CLI Playwright 2/2：公开 users/by-handle/batch、新旧路由、OIDC discovery、真实 OIDC 登录、Gallery 管理员页、
  Account display name/handle、`/media` WebP、无应用请求失败或 5xx。

## 发布顺序与已知边界

1. 发布 Foundation typed `SubjectKind`，再升级消费者依赖。
2. 发布 Identity API/Account 与 `@yueli/identity-nuxt` 新包。
3. 各站升级 Identity Nuxt 锁定依赖并验证 `/api/v1/users*`。
4. 发布并采用共享 Asset Nuxt `/media` route 后，删除消费者中的临时本地 route copy。

Gallery 当前安装的 `@yueli/identity-nuxt@0.1.0` 仍会对已删除的 `/api/v1/profiles/{sub}` 发起补充请求；旧包会把
404 降级，因此本地登录和管理流程仍可通过，但发布前必须升级依赖。禁止为此恢复旧接口。

## 复用规则

后续评审先阅读本文件、[当前合同审计](current-contract-audit.md) 与
[主流实践调研](user-identity-market-research.md)。只有协议、产品合同或外部一方资料发生实质变化时才重新调研。
