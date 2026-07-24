# 身份服务

- 生命周期：活跃的平台服务
- 权威来源：Catalog `platformServices.identity`、迁移和生成的 OpenAPI
- 消费者：Account、全部产品 BFF/API、Asset 及其他平台服务
- 验证：`go test ./services/identity/...`

Identity 是站群唯一身份提供方，负责账户、凭据、公开资料、登录会话、角色、个人访问令牌、OAuth2/OIDC 客户端与签名密钥。产品服务拥有领域权限和内容；它们消费 Identity subject 与 scope，不能复制用户记录或自行实现登录。

## 运行模型

- PostgreSQL 持久保存身份、凭据、资料、Identity 会话、OIDC 协议状态、刷新令牌、客户端和密钥。
- Redis 是会话热缓存，也是限流与锁定依赖；Redis miss 时可从 PostgreSQL 恢复有效 Identity 会话。
- `ory/fosite` 提供 OAuth2/OIDC 协议状态机。公共客户端使用授权码与 PKCE S256；短期访问令牌为 RS256 JWT，消费者通过 JWKS 本地验证。
- 刷新令牌持久化并轮换；单会话退出、全部退出和 RP 发起退出会撤销对应刷新令牌范围。
- Account 是 Catalog 声明的每环境唯一配套界面，不是第二个身份权威。

长期实现契约见 `flightdeck/knowledge/auth/identity-oidc-provider-boundary.md` 与 `flightdeck/knowledge/auth/login-session-durability.md`。

## 接口面

- `/api/v1/auth/*`：注册、密码/passkey 登录、MFA/恢复/step-up、验证、密码重置/修改和可选 Google 登录。
- `/api/v1/session/*`：当前身份、资料与媒体更新、凭据和会话/设备管理。
- `/api/v1/account/passkeys*`、`/api/v1/account/mfa*`：认证器 enrollment、列表、重命名、恢复和安全移除。
- `POST /api/v1/session/privacy/erasure`：用稳定幂等键、请求时间和客户端生成的状态能力令牌发起删除。
- `POST /api/v1/privacy/requests/{id}/status`：凭状态能力令牌继续驱动并读取聚合结果，账号删除后仍可用。
- `POST /api/internal/privacy/owner`：供实例内协调协议调用的 Identity Owner Host，要求 `privacy:owner`。
- `/api/v1/profiles*`：公开资料查询。
- `/api/v1/pat*`：个人访问令牌生命周期和验证。
- `/api/v1/admin/*`：用户、能力、provider 运维，以及 Account 使用的限域平台服务代理。
- `/.well-known/openid-configuration`、`/oauth2/*` 与 `/oauth2/jwks.json`：不使用平台信封的标准协议端点。
- `/healthz`、`/readyz`、`/api.json`、`/swagger`：运维与接口发现。

字段级接口真值以生成的 OpenAPI 为准。

## 目录地图

- `api/v1/`：业务请求与响应契约。
- `cmd/identity/`：依赖组合、路由和启动验证。
- `internal/logic/`：身份、会话与凭据领域行为。
- `internal/authentication/`：认证上下文、passkey/TOTP/recovery/step-up 深模块及协议 adapter seam。
- `internal/password/`：统一 Unicode 密码策略、blocklist 与 Argon2id/bcrypt 渐进迁移。
- `internal/identitymaintenance/`：短生命周期安全状态的有界 retention 清理。
- `internal/oidc/`：Fosite store、PG 事务 adapter、claims 和密钥。
- `internal/dao/`、`internal/repo/`：持久化与缓存 seam。
- `internal/controller/`：业务、OIDC 与 OAuth 登录 HTTP seam。
- `internal/identityprivacy/`：实例本地 Coordinator、最终化 Identity Owner 与状态能力映射。
- `manifest/config/`：配置模板，真实配置被 Git 忽略。
- `manifest/sql/migrations/`：唯一 schema 历史权威。

## 开发

正常开发使用 Catalog 生命周期，它会一致地 provision OIDC 客户端和共享本地账户（`test@example.com` / `Yueli local development 2026`）：

```powershell
pnpm platformctl dev up --file catalog/overlays/local.yaml --root . docs-main
pnpm platformctl dev status --root .
```

隔离开发时，把 `manifest/config/config.example.yaml` 复制为忽略的 `config.yaml`。必须提供 PostgreSQL、Redis 和至少 32 字节且稳定的 `GF_OIDC_GLOBALSECRET`；`oidc.issuer` 必须等于外部可访问的 Identity origin。

```powershell
go run ./services/identity/cmd/identity
go test ./services/identity/...
```

DAO 集成测试使用 `TEST_PG_LINK` 和 `TEST_REDIS_ADDR`；完整迁移 lifecycle 使用
`IDENTITY_MIGRATION_PG_*` 创建并删除隔离临时库。不要把整套历史 integration suite 指向长期
Identity 数据库。生产 wiring 绝不能把 PG 会话/OIDC store 替换为内存实现。现代认证的长期契约见
`flightdeck/knowledge/auth/identity-authentication-system.md`。

Privacy 的远程 Owner URL 与 confidential OAuth client 在 `privacy.*` 配置，并由部署 Catalog 的
`privacy:owner` service client 自动生成。每个独立站点使用 `site.<slug>` Owner key；协调器只冻结当前部署
实际存在的 Owner，不假设所有产品或 Notification 永远同部署。Identity 只保存请求、任务和最小回执；
Blog 与 Notification 仍直接拥有数据。Identity Owner 被声明为 finalizer，只有其他 Owner 全部终态后才
删除账号。独立 `identity:privacy` Work 实例会持续恢复 Owner 不可用或响应丢失的请求。
`PRIVACY_CONSUMER_PG_DSN` 可运行真实 PostgreSQL 编排测试。
