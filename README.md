# 身份服务

- 生命周期：活跃的平台服务
- 权威来源：本仓源码、版本化迁移、`doctor.yaml` 和生成的 OpenAPI
- 消费者：Account、全部产品 BFF/API、Asset 及其他平台服务
- 验证：进入本目录并设置 `GOWORK=off` 后执行 `go test ./...`

Identity 是站群唯一身份提供方，负责账户、凭据、公开资料、登录会话、角色、个人访问令牌、OAuth2/OIDC 客户端与签名密钥。产品服务拥有领域权限和内容；它们消费 Identity subject 与 scope，不能复制用户记录或自行实现登录。

## 运行模型

- PostgreSQL 持久保存身份、凭据、资料、Identity 会话、OIDC 协议状态、刷新令牌、客户端和密钥。
- Redis 是会话热缓存，也是限流与锁定依赖；Redis miss 时可从 PostgreSQL 恢复有效 Identity 会话。
- `ory/fosite` 提供 OAuth2/OIDC 协议状态机。公共客户端使用授权码与 PKCE S256；短期访问令牌为 RS256 JWT，消费者通过 JWKS 本地验证。
- 刷新令牌持久化并轮换；单会话退出、全部退出和 RP 发起退出会撤销对应刷新令牌范围。
- Account 是本仓唯一配套界面，不是第二个身份权威。

## 接口面

- `/api/v1/auth/*`：注册、密码/passkey 登录、MFA/恢复/step-up、验证、密码重置/修改和可选 Google 登录。
- `/api/v1/session/*`：当前身份、资料与媒体更新、凭据和会话/设备管理。
- `/api/v1/account/passkeys*`、`/api/v1/account/mfa*`：认证器 enrollment、列表、重命名、恢复和安全移除。
- `POST /api/v1/session/privacy/erasure`：用稳定幂等键、请求时间和客户端生成的状态能力令牌发起删除。
- `POST /api/v1/privacy/requests/{id}/status`：凭状态能力令牌继续驱动并读取聚合结果，账号删除后仍可用。
- `POST /api/internal/privacy/owner`：供实例内协调协议调用的 Identity Owner Host，要求 `privacy:owner`。
- `/api/v1/profiles*`：公开资料查询。
- `/api/v1/pat*`：个人访问令牌生命周期和验证。
- `POST /api/v1/account/publisher-attestations`：最近认证用户对精确消费者、命名空间与制品摘要签发发布者证明。
- `/api/v1/publisher/verification-keys`、`/api/v1/publisher/trust-manifest`：在线 key 发现与 offline-root
  签名的版本化离线信任快照；普通 key 列表不能代替 trust manifest。
- `/api/v1/account/github-bindings*`：最近认证用户发起 GitHub App user authorization，查看绑定历史并解绑；
  callback 只信任 authenticated `GET /user.id`，不复用登录 credential。
- `POST /api/v1/webhooks/github`：验证 `X-Hub-Signature-256` 后处理必收的
  `github_app_authorization/revoked`，停止未来 GitHub 授权但保留历史。
- `POST /api/internal/publisher/github-submissions`：要求
  `publisher:github-submission:authorize` 的 Registry 服务验证 active stable-ID binding、精确
  Publisher Attestation 与 repo/PR/commit provenance，并生成可保存的注册清单。
- `/api/v1/admin/*`：用户、能力、provider 运维，以及 Account 使用的限域平台服务代理。
- `/.well-known/openid-configuration`、`/oauth2/*` 与 `/oauth2/jwks.json`：不使用平台信封的标准协议端点。
- `/healthz`、`/readyz`、`/api.json`、`/swagger`：运维与接口发现。

字段级接口真值以生成的 OpenAPI 为准。
Account 使用的错误码目录位于 `contracts/errors/catalog.json`，由
`go run ./cmd/errorcatalog` 生成；CI 使用 `go run ./cmd/errorcatalog --check` 拒绝目录漂移。

## 目录地图

- `api/v1/`：业务请求与响应契约。
- `cmd/identity/`：依赖组合、路由和启动验证。
- `internal/logic/`：身份、会话与凭据领域行为。
- `internal/authentication/`：认证上下文、passkey/TOTP/recovery/step-up 深模块及协议 adapter seam。
- `internal/password/`：统一 Unicode 密码策略、blocklist 与 Argon2id/bcrypt 渐进迁移。
- `internal/identitymaintenance/`：短生命周期安全状态的有界 retention 清理。
- `internal/oidc/`：Fosite store、PG 事务 adapter、claims 和密钥。
- `internal/githubbinding/`：独立 GitHub ownership aggregate、GitHub App PKCE adapter、binding history
  与 PR submission authorization；不属于登录凭据模型。
- `internal/dao/`、`internal/repo/`：持久化与缓存 seam。
- `internal/controller/`：业务、OIDC 与 OAuth 登录 HTTP seam。
- `internal/identityprivacy/`：实例本地 Coordinator、最终化 Identity Owner 与状态能力映射。
- `manifest/config/`：配置模板，真实配置被 Git 忽略。
- `manifest/sql/migrations/`：唯一 schema 历史权威。

## 开发

仓库自治清单位于 `doctor.yaml`。安装统筹仓库提供的 Doctor CLI 后，在本仓执行：

```text
doctor check
doctor test
doctor up --detach
doctor status --check
doctor logs identity
doctor down
```

`check` 不启动进程或修改数据库；缺少运行配置、数据库 URL 或可选 Asset/Notification 地址时会明确给出警告。
`up` 要求先把 `manifest/config/config.example.yaml` 复制为忽略的 `config.yaml`。必须提供 PostgreSQL、Redis
和至少 32 字节且稳定的 `GF_OIDC_GLOBALSECRET`；`oidc.issuer` 必须等于外部可访问的 Identity origin。

```powershell
$env:GOWORK = "off"
go run ./cmd/publishertrust `
  -issuer http://localhost:8081 `
  -key-ring .data/publisher/key-ring.json `
  -root-key .data/publisher-offline/root-key.pem `
  -root-public .data/publisher/trust-root.json `
  -manifest .data/publisher/trust-manifest.json
go run ./cmd/identity
go test ./...
pnpm --dir account install --frozen-lockfile
pnpm --dir account test
pnpm --dir account typecheck
pnpm --dir account build
```

`publishertrust` 是离线运维命令。Identity runtime 只配置 leaf signing key、root 公钥和已签 manifest；
offline root 私钥不得复制到运行环境。生产环境使用 KMS/HSM leaf adapter，并通过受控发布流程递增
`manifestVersion`。

生产可设置 `publisher.mode=secret-pem` 并由 secret manager 注入
`GF_PUBLISHER_PRIVATEKEYPEM`；该模式不写本地 key 文件。KMS/HSM 客户端通过标准
`crypto.Signer` 接入 `NewCryptoSignerProvider`，且未知 mode 会令进程启动失败，不会回退到 local-file。

Publisher key rotation 是两阶段操作：

1. 管理员用 action `identity.admin.publisher.key.prepare`、resource `publisher:key-ring` 的 step-up proof
   调用 `POST /api/v1/admin/publisher/keys`，生成 `preactive` key。
2. 离线运行 `publishertrust -version N` 签出仍为 preactive 的 manifest，使用 action
   `identity.admin.publisher.manifest.apply` 和 resource
   `publisher:trust-manifest:<响应 snapshotHash>` 应用并预发布。
3. 等待消费者刷新窗口后，离线增加 `-activate-key <kid> -version N+1`，再次应用；旧 active 自动转为
   retired。`-disable-signing` 会生成零 active key 的更高版本 manifest，从而显式停用未来签发。

所有 manifest 更新必须提高版本；retired、compromised 或 revoked key 不能重新激活。历史公钥始终保留。

GitHub Binding 生产配置必须同时提供 `githubBinding.clientId`、`clientSecret`、`redirectUrl` 与
`webhookSecret`，缺一即拒绝启动。授权 state 一次性且绑定当前 Identity session，PKCE verifier 在数据库中
加密；GitHub user token 只用于 callback 内的 `GET /user`，不会落库，并在完成后 best-effort 撤销。GitHub
改名只刷新展示快照；冲突不会自动转移，解绑或 GitHub 侧撤销只关闭 active row，后续重新授权会追加新历史。
Registry 的用户名、email、repository owner 或 commit author 都不能替代 stable account ID binding。
账号执行隐私删除时，Identity 会先把 binding 关闭并清除 GitHub account ID、node ID、login 与 avatar；
Publisher subject、已签证明和 Registry 已保存的历史 submission manifest 仍按各自保留策略存在，数据库外键不会阻断删除。

DAO 集成测试使用 `TEST_PG_LINK` 和 `TEST_REDIS_ADDR`；完整迁移 lifecycle 使用
`IDENTITY_MIGRATION_PG_*` 创建并删除隔离临时库。不要把整套历史 integration suite 指向长期
Identity 数据库。生产 wiring 绝不能把 PG 会话/OIDC store 替换为内存实现。

Privacy 的远程 Owner URL 与 confidential OAuth client 在 `privacy.*` 配置，并由部署组合提供
`privacy:owner` service client。每个独立站点使用 `site.<slug>` Owner key；协调器只冻结当前部署
实际存在的 Owner，不假设所有产品或 Notification 永远同部署。Identity 只保存请求、任务和最小回执；
Blog 与 Notification 仍直接拥有数据。Identity Owner 被声明为 finalizer，只有其他 Owner 全部终态后才
删除账号。独立 `identity:privacy` Work 实例会持续恢复 Owner 不可用或响应丢失的请求。
`PRIVACY_CONSUMER_PG_DSN` 可运行真实 PostgreSQL 编排测试。
