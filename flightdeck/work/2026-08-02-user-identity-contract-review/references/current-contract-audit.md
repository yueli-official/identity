# Identity User 合同与消费者审计

审阅日期：2026-08-02

## 结论

当前 UUID 并不是不合规的 OIDC `sub`，也没有熵或可枚举性缺陷；真正的问题是同一个数据库 UUID 同时承担了
内部主键、第一方 OIDC subject、跨服务所有权键、公开 Profile 查询键、公开作者 URL 和管理员操作键。它把
存储 implementation 变成了全站接口，也让本来可变的 handle 没有形成真实产品合同。

如果平台确实尚未发布，建议发布前完成一次协调迁移：保留现有 UUID 作为 Identity 内部主键；新增 128-bit
随机、带 `usr_` 前缀的公开 user key；第一方站点有意共享同一个稳定 subject 范围；handle 与 display name
独立。不要仅把 UUID 换一种编码，也不要让 handle 成为外键或授权键。

## 当前标识矩阵

| 概念 | 当前实现 | 当前消费者 | 判断 |
| --- | --- | --- | --- |
| 内部 User 主键 | `identities.id UUID DEFAULT gen_random_uuid()` | Identity 全部 FK | 合理，保留内部使用；不值得为 UUIDv7 重写已有 PK |
| OIDC user subject | `id.ID` 直接写入 ID/access token | 全部产品 BFF/API | 规范合规，但与内部 PK 耦合；Discovery 只声明 `public` |
| 机器 token subject | `client_id` 写入同一个 `sub` 字段 | 平台服务 | User 与 Client 没有强类型区分，依赖 scope 防止误用 |
| 跨服务 User key | JWT `sub` 原样保存为 `TEXT` | Gallery、Shop、Resource、Docs、Blog、Asset、Commerce、Nav | 已是平台级持久合同，改变值需要协调数据迁移 |
| 公开 Profile key | `/api/v1/profiles/{id}` 的 `id` | Identity Nuxt、Blog | 直接暴露内部 UUID |
| 公开作者 URL | Blog `/author/{id}` | Blog 页面与文章署名 | 用户直接看到 UUID；缺资料时 UI 展示 UUID 前 8 位 |
| handle | `user_profiles.username TEXT` | Account 编辑/管理页面、OIDC `preferred_username` | 可空、大小写敏感唯一；没有解析、改名、历史或公开 URL 合同 |
| display name | `user_profiles.display_name` | OIDC/Profile/Blog/Account | 角色正确，但缺少统一长度和字符策略 |
| 外部登录 identity | `credentials_oauth(provider, provider_uid)` | 当前 Google 登录 | 稳定 provider UID 优于 email；扩展通用 OIDC 时应升级为精确 `(issuer, sub)` |
| GitHub 发布者 binding | GitHub stable account ID / node ID + Identity UUID | Publisher/Registry | 与登录 credential 分离，方向正确 |
| WebAuthn user handle | 独立随机 bytes | Passkey | 已正确与 username/display name 分离 |

## 消费者影响

- Foundation 的 `auth.Principal.Subject` 是不透明 `string`，产品仓没有 UUID 类型假设。
- Gallery、Shop、Resource、Docs、Blog、Nav 的 authorization/audit/ownership 列均使用 `TEXT` subject。
- Asset 的 `owner_id`、grant `subject_id` 使用 `TEXT`。
- Commerce 的 `buyer_sub`、delivery grant 使用 `TEXT`。
- Blog 额外持久化 `author_id`、comment/like/bookmark `user_id`，并通过 Identity Profile 接口解析展示资料。
- Identity Nuxt 使用 token `sub` 调 `/api/v1/profiles/{sub}`；Blog 是另一直接 Profile 客户端。
- Workspace 和各产品 devseed/Compose 中存在固定 UUID admin subject。
- Identity 自身仍有 `publisher_attestations.publisher_subject UUID`、privacy finalizer 的 `::uuid` 转换以及大量
  session/step-up/admin 接口直接把 external subject 当内部 UUID；这些是迁移的主要 Identity 内部影响面。

消费者字段大多是 `TEXT`，所以改成 `usr_...` 不需要修改其列类型；但已有值、fixture、授权关系、作者内容和
订单归属必须同时改写。不能只切 token `sub`，否则所有产品会把同一个人当成新用户。

## 发现

### F1 — 高：Profile 写接口允许不受控公开 URL 与社交链接

- [`api/v1/account.go`](../../../../api/v1/account.go) 的通用 Profile 更新允许直接提交 `avatarUrl`、`coverUrl` 和任意
  `socialLinks[].url`。
- [`internal/logic/profile.go`](../../../../internal/logic/profile.go) 只 trim 字符串，没有 scheme、host、长度、数量或
  控制字符校验。
- 仓库已经有受控 Avatar/Cover 上传接口，但通用更新可以绕过 Asset，制造外部跟踪图并让保存的 Asset ID 与 URL
  来源失配。
- Blog 作者页把 social URL 直接交给导航控件。当前至少允许非 HTTP(S) scheme 和钓鱼目标进入公开页面；应只接受
  规范化 `https`/必要时 `http` URL，并验证最终渲染是否会把危险 scheme 升级为可执行 XSS。

建议从通用 Profile 更新中删除 avatar/cover URL；图片只经 Asset upload/commit。所有公开文本与链接由一个
Profile policy 统一限制长度、数量、scheme 和控制字符。

### F2 — 高：username 当前不是可用的 handle 合同，且空值会产生生产数据库冲突

- Account 把 username 描述为“用于个性化地址”，但没有 `/@handle`、handle lookup 或公开 Profile username 字段。
- 逻辑把空 username 保存成 `""`；数据库唯一索引只排除 `NULL`，因此第一个空字符串会占用唯一值，后续用户保存
  空 username 会触发唯一约束错误。
- Memory adapter 不实现 username 唯一性，所以现有测试看不到该生产差异。
- 当前唯一性大小写敏感，且没有 grammar、canonicalization、保留词、改名频率、历史、tombstone 或回收政策。
- 唯一约束错误没有映射为稳定的公开 Problem，可能表现为 500。

建议不要继续补丁式修复 `username` 字段；建立独立 handle 深模块，首版采用 lowercase ASCII、数据库 canonical
唯一值、保留词和永久历史占用。空 handle 必须真正存 `NULL` 或没有 current row。

### F3 — 高：公开 Profile 查询把故障伪装成不存在，并提供无上限 N+1 路径

- 单用户 Controller 对任何错误都返回 200 空壳，而不是只处理 `ErrIdentityMissing`；数据库不可用也会被伪装成空用户。
- 批量接口没有 ID 数量、总长度或去重上限，`PublicProfiles` 对每个输入逐条查询并吞掉全部错误。
- Blog 客户端进一步吞掉所有网络/协议错误。Identity 故障会静默变成作者资料消失，直接污染页面和缓存判断。
- 单查返回空壳、批量查省略缺失项，所谓“不泄露存在性”策略本身也不一致。

建议 Profile resolution 成为深模块接口：单次批量查询、有界输入、明确 `found/not found/unavailable`，HTTP adapter
分别返回正常资源、404 与 5xx；客户端可做短时 stale cache，但不能把故障缓存成用户不存在。

### F4 — 高：管理员“删除”与隐私删除是两套相冲突的生命周期

- Admin delete 只把状态设为 `deleted`，撤销会话并通过 partial email index 释放 email；它不清理 Profile PII。
- 公开 Profile 查询不检查 Identity status，所以 admin-deleted 用户的 display name、bio、social links、avatar/cover
  仍可继续公开。
- Privacy erasure finalizer 则最终硬删除 Identity 并级联 Profile。相同“删除”词对应两套可见性和数据语义。
- 被释放的 email 可以创建新主体，而旧主体的公开资料和 handle 仍存在，增加客服、冒充和数据治理歧义。

建议把 `disabled/suspended` 与 `deletion_pending/deleted` 分开。管理员若只是封禁应叫 suspend；真正 delete 应进入
统一 erasure/finalization 流程，并在进入 deletion_pending 时立即停止公开 Profile。不可复用 user key、OIDC
subject 和历史 handle 只保留最小 tombstone。

### F5 — 中高：用户创建与角色授予不是一个可靠结果

- 普通注册和 OAuth 隐式注册都把默认 `user` 角色授予写成 best-effort，失败仍返回成功并记录“已授予”审计。
- Admin create 先创建用户，再逐个授予额外角色；未知角色或存储失败会返回错误，但用户及此前角色已经落库。
- 这会产生“调用方认为失败、数据库已有部分用户”的重试与权限歧义。

建议 User creation interface 返回一个原子结果：先验证完整 role set，在一个 PostgreSQL transaction 中创建主体、
Profile、credential 与初始 roles；Memory adapter 通过同一行为测试。

### F6 — 中高：Memory 与 PostgreSQL adapter 的合同漂移使绿测试失真

- `NewIdentityInput.ID` 在 Memory 生效，PostgreSQL implementation 完全忽略。
- locale 空值在 PostgreSQL 被替换为 `zh-CN`，Memory 保留空字符串。
- PostgreSQL 的 deleted email 可重用，Memory 的 `byEmail` 不释放。
- PostgreSQL 有 username 唯一约束，Memory 没有。
- Profile/管理员生命周期缺少针对 PostgreSQL 语义的 integration 行为测试。

两个 adapter 已构成真实 seam；应以 User module interface 为测试面，建立同一套 adapter contract tests，而不是让
大而全的 Memory `Store` 定义事实。

### F7 — 设计：UUID 形状不是错误，标识职责混用才是

- UUIDv4 有足够随机性，符合 OIDC `sub` 要求；仅因“丑”而替换为另一个编码没有架构收益。
- 当前 Blog 确实把 UUID 放进公开作者 URL，Account 管理详情也显示内部 UUID，因此审美问题有真实用户触点。
- OpenAPI 中 Register/Login/Me/Profile/Admin 都只叫 `id` 或 `identityId`，没有说明哪一个是内部、公开或协议标识。
- OIDC Discovery 只支持 public subject；第一方跨站共享是当前业务需要，但未来第三方 client 没有 pairwise 隔离。

建议保留内部 UUID，新发 `usr_` public key。第一方 OIDC subject 可以有意等于该 public key，但 subject assignment
必须显式持久化；未来第三方按 sector 生成 pairwise subject。handle 只做人类路径。

### F8 — 中：机器身份与用户身份共用无类型 `sub`

- Client Credentials token 把 `client_id` 写进 `sub`；Foundation `Principal` 只有 `Subject string`，没有 subject kind。
- 多个产品 Controller 把 `principal.Subject` 直接用作 owner/buyer/user key。
- 当前 audience/scope 配置降低了误用概率，但一次 scope 配错即可把机器 client ID 持久化为“用户所有权”。

建议 access token 明确携带并验证 `subject_kind=user|client|guest`，Foundation Principal 提供 typed subject；所有
human-only interface 同时检查 token grant/subject kind，不能只因为 `sub` 非空就认作 User。

### F9 — 中：OpenAPI 没有表达 ID 与 Profile 的真实约束

生成合同把全部 ID、handle、URL、locale、display name 都写成无约束 `string`；Profile batch 是无上限 CSV；
Register/Login/Me 直接返回内部 UUID，但 schema 没有 format、pattern、maxLength 或 opaque 说明。消费者只能从实现
和示例猜接口。

建议新 v1 将 user key 声明为不透明、区分大小写的字符串并预留长度；handle、URL、文本、数组和 batch 都有明确
限制。业务 API 保持 `/api/v1`，标准 OIDC discovery/authorize/token/userinfo 不机械增加版本路径。

## Codebase Design 结论

当前 User 行为散落在通用 `logic.Service`、超宽 `repo.Store`、Profile/Register/Admin 文件、DAO 和 HTTP Controller。
调用方必须知道 UUID、status、Profile 缺失、role best-effort、username 空值等 implementation 细节；这是浅模块，
没有给调用方足够 leverage，也缺乏 locality。

建议建立一个 User 深模块：

- 外部 interface 只表达 create/current/update-profile/resolve-public/change-lifecycle 等 User 行为与明确结果。
- 模块内部拥有 UserID、PublicUserKey、OIDCSubject、Handle 等强类型和全部规范化/冲突/状态规则。
- PostgreSQL 与 Memory 是同一 repository seam 的两个 adapter；共享合同测试。
- HTTP/OIDC/Account 只是 wire adapter，在 seam 处一次性把 public key/subject 解析到内部 UserID。
- Profile batch、handle history、subject assignment 和生命周期事务全部隐藏在 implementation 内，消费者不复制规则。

删除这个模块时，上述复杂性会重新散落到 Controller、DAO 和所有产品；因此它是能通过 deletion test 的深模块，
而不是给现有文件再包一层转发。

## 推荐目标合同

### 标识

- `users.id`：保留现有 UUIDv4 作为内部 PK/FK；不为追求 UUIDv7 重写已有关系。
- `users.user_key`：`usr_` + 128-bit CSPRNG 的 22 字符 base64url，无时间/分片/业务语义，永久不可复用。
- 第一方 `sub`：显式 subject assignment，可有意使用 `user_key`；所有第一方产品继续共享同一 subject sector。
- 第三方 `sub`：默认 pairwise，按稳定 sector 持久化/生成。
- `handle`：首版 lowercase ASCII 3..30，独立 current/history，历史默认不分配给其他主体。
- `display_name`：可重复 Unicode 展示文本，不参与登录、URL、FK 或授权。

### 对外形状

```text
GET /api/v1/users/{userKey}
GET /api/v1/users?ids={bounded-list}
GET /api/v1/users/by-handle/{handle}

GET /@{handle}       # canonical 人类页面
GET /u/{userKey}     # 永久链接
```

```json
{
  "id": "usr_7Yq4fP9m2KxVd3Nw8Rc1Ag",
  "handle": "alice",
  "displayName": "Alice 陈",
  "avatar": { "mediaKey": "..." },
  "cover": { "mediaKey": "..." },
  "bio": "...",
  "socialLinks": []
}
```

公开 Profile 不应持久化任意完整 Asset URL；保存 `assetId/mediaKey`，由统一 Asset media helper 生成 rendition URL。

### 版本与迁移

- 若确认尚未发布，删除 `/api/v1/profiles*` 和内部 UUID wire 字段，不保留 redirect、deprecated handler、双读或 fallback。
- 新正式业务合同从 `/api/v1/users*` 开始；以后 additive 变化留在 v1，breaking change 才新增 v2。
- OIDC issuer、`sub`、token 和 discovery 是身份迁移合同，不能靠普通 API v2 自动隔离。
- 切换 `sub` 前生成 `internal UUID -> user_key` 映射，并在 Identity、Publisher、Blog、Commerce、Asset、authorization、
  audit、Workspace fixtures 中协调改写；如果所有数据库只是本地 fixture，则优先一次 reset/reseed，不制造迁移兼容层。

## 推荐顺序

1. 先修 F1–F6 的确定性行为缺陷并补 adapter contract tests。
2. 固化 UserID/PublicUserKey/OIDCSubject/Handle 类型与数据库 migration。
3. 协调切换第一方 OIDC `sub` 和全部消费者持久化值；删除旧 UUID wire 合同。
4. 新建 `/api/v1/users*`，迁移 Identity Nuxt 与 Blog；增加 `/@handle`、`/u/{userKey}`。
5. 最后增加第三方 pairwise subject；不要让它阻断第一方发布。

主流规范与平台证据见 [用户标识与账户合同调研](user-identity-market-research.md)。
