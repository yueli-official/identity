# 用户标识与账户合同：规范、主流身份系统与大型平台一手资料调研

> 调研日期：2026-08-02  
> 资料范围：OpenID Connect、OAuth、SCIM、UUID/PRECIS/Unicode 规范；Keycloak、Auth0、ZITADEL 官方资料；Google、GitHub、X、Discord、Slack、AT Protocol 官方文档。  
> 资料原则：只使用规范、官方文档或官方源码仓库，不使用二手博客。本文不评价当前仓库实现；“项目建议”需要再与本地代码审阅结果交叉验证。

## 结论摘要

1. **内部数据库主键、OIDC `sub`、公开用户 key、handle、display name 应当分离。** 这不是为了追求抽象，而是因为它们的作用域、可见性、稳定性、隐私和变更规则不同。SCIM 已明确把服务端稳定 `id`、客户端域中的 `externalId` 和登录名 `userName` 定义为不同属性；OIDC 则只承诺 `(iss, sub)` 是 RP 可依赖的稳定用户标识。
2. **UUID 作为 `sub` 在规范上完全合法。** OIDC 只要求 `sub` 在 issuer 内唯一、永不重新分配、区分大小写、最长 255 个 ASCII 字符。问题不在“UUID 丑”，而在它是否直接暴露了内部主键、是否被跨客户端复用造成不必要关联、是否会在迁库或删号重建时变化。
3. **公开 `sub` 和 pairwise `sub` 应按客户端信任边界选择。** 同一组织、确实需要统一账号联动的第一方站点可以共享一个 subject 范围；第三方或相互不应关联的客户端应使用 pairwise subject。不要在上线后静默切换 subject 类型或 sector，否则 RP 会把同一个人视为新用户。
4. **人看的 URL 和机器合同应双轨。** 人类资料页可用 `/@{handle}`；API、外键、事件和长期引用应使用稳定的公开 `userKey`。可额外提供 `/u/{userKey}` 作为永久链接。GitHub、X、Discord 和 AT Protocol 都证明了“稳定 ID + 可变用户名/显示名”是成熟做法。
5. **当前若尚未发布，旧接口应直接删除，不做兼容、重定向或 fallback；但正式接口从第一天就进入 `/api/v1`。** 发布后，v1 内只做兼容性增加；破坏性变化进入 `/api/v2` 并与 v1 并存一段迁移期。OIDC discovery、issuer、authorize/token 等协议端点不应机械套用业务 API 的版本路径。
6. **建议的目标形态**：内部 `users.id` 使用数据库友好的 UUIDv7；公开 `users.public_key` 使用 CSPRNG 生成的 128 位、不含业务语义的短字符串（例如 `usr_` + 22 位无填充 base64url）；OIDC subject 单独持久化；handle 单独建表并保留历史；display name 为可重复的 Unicode 文本；外部身份以精确的 `(issuer, subject)` 绑定到本地用户。

## 1. 先固定术语：五种标识不是同一个东西

### 1.1 规范事实

- OIDC Core 规定 `sub` 是 issuer 内“本地唯一且永不重新分配”的 End-User 标识，区分大小写，最长 255 个 ASCII 字符；同时明确指出，RP 唯一可以依赖为稳定用户标识的是 **`iss` 与 `sub` 的组合**，email 等其他 claim 可能变化或被重新使用。[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0-final.html#IDToken)、[OIDC Core §5.7](https://openid.net/specs/openid-connect-core-1_0-final.html#ClaimStability)
- SCIM Core 把 `id` 定义为服务提供方签发、稳定、不可重新分配、只读且不透明的标识；`externalId` 由 provisioning client 签发并限定在该客户端域中；`userName` 是用户登录名。三者是明确分离的合同。[RFC 7643 §3.1](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1)、[RFC 7643 §4.1.1](https://www.rfc-editor.org/rfc/rfc7643.html#section-4.1.1)
- PRECIS 的 nickname 规范提醒：昵称或 handle 是附着在更稳定底层身份上的人类友好名称，认证和授权必须基于底层身份，而不是 nickname。[RFC 8266 §6.1](https://www.rfc-editor.org/rfc/rfc8266.html#section-6.1)
- OAuth 2.0 本身解决授权而不是登录身份；JWT access token profile 进一步说明，用户参与的授权中 `sub` 通常代表资源所有者，而 client credentials 中 `sub` 通常代表客户端。因此资源服务不能无条件把任何 access token 的 `sub` 都解释成“用户 ID”。[RFC 6749 §1](https://www.rfc-editor.org/rfc/rfc6749.html#section-1)、[RFC 9068 §2.2](https://www.rfc-editor.org/rfc/rfc9068.html#section-2.2)

### 1.2 大型平台事实

- Google 明确要求应用使用 `sub` 关联用户，不要使用 email；Google 账号的 email 可变化，而 `sub` 永不变化或复用。[Sign in with Google: OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect)
- GitHub 同时返回 `login`、数字 `id`、`node_id` 和显示名称，并提供“按 durable user ID 获取用户”的专门端点；官方文档明确写明 `login` 会随用户名修改而变化。[GitHub REST users: Get a user using their ID](https://docs.github.com/en/rest/users/users#get-a-user-using-their-id)
- X 的 User 对象把 `id`、`username` 和 `name` 分成三个字段，并支持分别按唯一 ID 和 username 查找。[X User Lookup](https://docs.x.com/x-api/users/lookup/introduction)、[X Get User by ID](https://docs.x.com/x-api/users/get-user-by-id)
- Discord User 对象把全局 Snowflake `id`、并不全平台唯一的 `username`、可空的 `global_name` 分开。[Discord User Resource](https://docs.discord.com/developers/resources/user)
- AT Protocol 明确把 DID 定义为长期持久账号标识，把 handle 定义为可变、人类友好用户名；即使账号没有有效 handle，DID 仍是主身份。[AT Protocol DID](https://atproto.com/specs/did)、[AT Protocol Handle](https://atproto.com/specs/handle)

### 1.3 推断

把内部主键直接当作所有外部合同，短期字段少，但会把以下本来独立的变化绑死：

- 数据库迁移、分片和存储布局；
- 对外 API 标识格式；
- OIDC public/pairwise 隐私策略；
- 用户改 handle、改显示名；
- 外部 IdP 绑定、解绑和账号合并；
- 删号后的标识保留与重新注册。

### 1.4 项目建议：目标数据合同

| 概念 | 建议字段/实体 | 稳定性与可见性 | 建议用途 |
| --- | --- | --- | --- |
| 内部主体主键 | `users.id` | 永久；仅服务内部 | 数据库 PK/FK、事务、领域事件内部关联；建议 UUIDv7 |
| 公开用户 key | `users.public_key` | 永久、不可复用；可公开 | API `id`、跨服务引用、webhook、永久链接；建议 `usr_...` |
| OIDC subject | `oidc_subjects.sub` | 在 `(issuer, client/sector policy)` 中永久 | ID Token/UserInfo；不要动态读取 handle/email 生成 |
| 公开 handle | `user_handles.canonical_handle` | 可改、唯一、大小写规范化 | `/@handle`、@mention、搜索；不得用于授权或 FK |
| 显示名 | `profiles.display_name` | 可随时改、可重复、Unicode | UI 展示；不参与唯一性、登录或路由 |
| 外部身份 | `external_identities(issuer, subject)` | 绑定关系可增删；键本身精确匹配 | Google/GitHub/企业 IdP 登录映射 |

建议 API 对外仍把 `public_key` 命名为简单的 `id`，但 OpenAPI 必须写清楚它是稳定、不透明、区分大小写的字符串；内部主键不要序列化。领域代码中则使用 `UserID`、`PublicUserKey`、`OIDCSubject`、`Handle` 等不同类型，避免字符串误接。

## 2. UUID 作为 `sub` 到底有没有问题

### 2.1 事实

- OIDC 不规定 `sub` 的生成算法或美观形式。标准示例既有十进制字符串，也有较长的不透明字符串；合法条件是 issuer 内唯一、永不重新分配、最长 255 ASCII、区分大小写。[OIDC Core §2](https://openid.net/specs/openid-connect-core-1_0-final.html#IDToken)
- UUID 是 128 位标识。UUIDv4 有 122 个随机位；UUIDv7 的高 48 位是 Unix 毫秒时间戳，标准布局通常保留 74 位随机/单调数据。[RFC 9562 §5.4](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.4)、[RFC 9562 §5.7](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.7)
- RFC 9562 建议需要难猜的 UUID 使用 CSPRNG；它也明确把数据库索引局部性作为 UUIDv7 的设计动机之一。[RFC 9562 §6.9](https://www.rfc-editor.org/rfc/rfc9562.html#section-6.9)、[RFC 9562 §2.1](https://www.rfc-editor.org/rfc/rfc9562.html#section-2.1)
- Keycloak 官方管理指南提供 Pairwise Subject Identifier mapper，说明成熟身份服务器也不会假设 `sub` 必须等于本地用户主键。[Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/)

### 2.2 推断

因此，“UUID 看起来丑”不是替换 `sub` 的理由：`sub` 本就应被客户端当作不透明机器标识，通常不会出现在面向用户的 URL。真正需要审查的是：

1. 该 UUID 是否会在删除重建、导入、迁库或合并时变化；
2. 是否把同一个 UUID 发给了所有无关客户端，从而允许跨站关联；
3. 是否把内部存储主键暴露为长期外部合同；
4. 若是 UUIDv7，是否接受公开创建时间的大致泄露；若是 UUIDv1，是否存在时间或节点信息泄露；
5. 是否有任何业务代码解析 UUID 版本、时间或字符串结构。

### 2.3 项目建议

- **内部主键**：UUIDv7 很合适，便于分布式生成和数据库索引局部性。它不需要对外漂亮。
- **公开 user key**：另发 128 位 CSPRNG 随机值，并编码为无填充 base64url；16 字节会编码为 22 个字符。加类型前缀后的示例：`usr_7Yq4fP9m2KxVd3Nw8Rc1Ag`。前缀用于日志和人工排错，不承载版本、分片或时间语义。
- **OIDC `sub`**：单独持久化。public subject 可以是另一个稳定随机字符串；pairwise subject 按 sector 单独持久化，或采用规范允许的带密钥确定性算法。持久化映射比直接编码内部 UUID 更容易支持密钥轮换、迁库和 subject 策略审计。
- **不要只为缩短字符而降低熵。** API schema 可声明最大 64 或 128 字符，当前生成 26 字符左右；客户端必须把它视为不透明字符串。AIP-180 提醒，改变已发布字符串值的格式或扩大已承诺长度都可能破坏客户端。[AIP-180: Changing value format or construction](https://google.aip.dev/180#changing-value-format-or-construction)

## 3. Public 与 pairwise `sub`：隐私、联动和迁移

### 3.1 规范事实

OIDC 定义两种 subject 类型：

- `public`：同一用户对所有 Client 获得相同 `sub`；
- `pairwise`：同一用户对不同 Client/Sector 获得不同 `sub`，避免客户端在未经许可的情况下关联用户活动。

Pairwise 值必须按 Sector Identifier 确定性生成，对 OP 之外的任何参与方不可逆；同一管理域下的一组站点可通过 `sector_identifier_uri` 获得一致 pairwise subject，并可在 redirect URI 域名变化时保持稳定。[OIDC Core §8](https://openid.net/specs/openid-connect-core-1_0-final.html#SubjectIDTypes)、[OIDC Core §8.1](https://openid.net/specs/openid-connect-core-1_0-final.html#PairwiseAlg)

Keycloak 提供 Pairwise Subject Identifier mapper，是该策略在主流身份服务中的直接实现证据。[Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/)

### 3.2 权衡

| 策略 | 优点 | 风险/成本 | 适用范围 |
| --- | --- | --- | --- |
| Public subject | 第一方站点关联简单；审计和客服排障直接 | 所有 client 可互相串联同一人；内部 ID 更容易扩散 | 同一数据控制者、明确需要统一账号的可信第一方客户端 |
| Pairwise subject | 最小化跨 client 关联；第三方泄露一个 `sub` 不暴露其他关系 | RP 侧迁移、客服和跨产品合并需要 OP 映射；sector 配置必须稳定 | 第三方应用、不同业务/租户、相互不应共享身份图谱的客户端 |
| 共同 sector 的 pairwise | 同一产品族内一致，对其他 sector 隔离 | sector 一旦规划错误，后续迁移成本高 | 同一组织的一组前端站点，共享账号但不想向外部 client 暴露公共 subject |

### 3.3 项目建议

1. 第一方多站点如果确实共享同一账号域，可为它们定义一个明确的 first-party sector；不要因为域名不同就意外生成多个 subject。
2. 第三方 OAuth/OIDC client 默认 pairwise，每个独立控制者一个 sector。
3. client 注册时固化 `subject_type` 与 `sector_id`。上线后改变任一项会让 RP 收到新 `sub`，应被视为账号迁移，而不是普通配置修改。
4. 如必须迁移，先保留旧 `(issuer, sub)` 到本地主体的映射，提供受控 linking/reconciliation 流程，并逐客户端验证；不能仅把新旧值同时塞进一个 token 期待 RP 自行猜测。
5. `sub` 的隐私边界不能靠 API v2 修复：RP 的账号键是 `(iss, sub)`，改变 issuer 或 subject 都是身份迁移事件。

## 4. 公开用户 URL 与 API ID

### 4.1 市场事实

- GitHub 资料页以可读 username 为路径，但旧 profile URL 在改名后返回 404，旧 username 也可能被别人重新认领；与此同时，REST API 提供 durable numeric user ID 查询。这体现了“人类路径方便，但不是稳定机器键”。[GitHub Username changes](https://docs.github.com/en/account-and-profile/concepts/username-changes)、[GitHub Get user using ID](https://docs.github.com/en/rest/users/users#get-a-user-using-their-id)
- AT Protocol 直接警告：包含 handle 的 AT URI 不耐久；handle 变化或复用后，旧 URI 可能失效，甚至指向另一个仓库。DID 才是长期主体标识。[AT URI scheme](https://atproto.com/specs/at-uri-scheme)
- X 同时支持 `/2/users/{id}` 和按 username 查询；Discord API 按 Snowflake user ID 获取用户，而 username/display name 是资料字段。[X User Lookup](https://docs.x.com/x-api/users/lookup/introduction)、[Discord User Resource](https://docs.discord.com/developers/resources/user)
- Slack 曾专门要求开发者不要假设 user ID 必须以某个字符开头或符合固定组成，应用和数据库应把 ID 当不透明字符串。[Slack: Some user ID strings are changing](https://api.slack.com/changelog/2016-08-11-user-id-format-changes)

### 4.2 项目建议

建议同时提供三种明确用途的入口：

```text
GET /@alice                         # 人看的当前 canonical profile URL
GET /u/usr_7Yq4fP9m2KxVd3Nw8Rc1Ag # 稳定永久链接，可 302 到当前 /@handle
GET /api/v1/users/usr_7Yq4fP9m2KxVd3Nw8Rc1Ag
GET /api/v1/users/by-handle/alice   # 显式解析可变 handle
```

API 资源示例：

```json
{
  "id": "usr_7Yq4fP9m2KxVd3Nw8Rc1Ag",
  "handle": "alice",
  "displayName": "Alice 陈",
  "profileUrl": "https://example.com/@alice"
}
```

合同要求：

- `id` 始终是字符串，稳定、不可复用、区分大小写；调用方不得解析前缀、长度、时间或分片。
- `handle` 是当前可变别名；只有解析完成后得到的 `id` 才能进入缓存键、外键、ACL、事件和审计记录。
- 公开活动、帖子作者、关注关系等响应都携带稳定 `id`；可以同时冗余当前 handle/display name 便于渲染，但缓存更新以 `id` 为准。
- 网站 canonical URL 可用 handle；需要永久分享的场景提供 `/u/{id}`。不要把内部 UUID 放进 URL，也不要让 username 成为唯一 API 路径。

## 5. Handle：大小写、Unicode、规范化、保留词、改名与回收

### 5.1 规范与平台事实

- RFC 8265 为国际化 username 定义了 `UsernameCaseMapped` 与 `UsernameCasePreserved` 两种 profile，并倾向 case mapping 以降低误接受和用户混淆。CaseMapped 依次执行大小写映射、NFC 规范化和双向文本规则；这说明 Unicode handle 不能靠语言内置的简单 `lower()` 处理。[RFC 8265 §3.2](https://www.rfc-editor.org/rfc/rfc8265.html#section-3.2)、[RFC 8265 §3.3](https://www.rfc-editor.org/rfc/rfc8265.html#section-3.3)
- Unicode UTS #39 给出混合脚本、全脚本和字符 confusable 检测，以及 ASCII-only、single-script、highly restrictive 等限制等级；它建议标识尽可能使用 casefolded 形式以减少大小写变体。[Unicode UTS #39](https://www.unicode.org/reports/tr39/)
- AT Protocol handle 只允许 ASCII 字母、数字和连字符，大小写不敏感，并要求存储和 API 传输使用小写规范形式。[AT Protocol Handle](https://atproto.com/specs/handle)
- Keycloak 的外部身份 broker 默认把 username 转为小写；官方文档说明服务端 username 始终小写。[Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/)
- GitHub 改名后旧 username 可被他人认领，部分旧链接不会重定向；仓库重定向也可能在旧名被新用户占用后失效。这是 handle 回收会导致链接劫持和语义漂移的直接案例。[GitHub Username changes](https://docs.github.com/en/account-and-profile/concepts/username-changes)

### 5.2 项目建议：第一版采用 ASCII canonical handle

在没有明确的全球多文字 handle 需求前，建议把 Unicode 留给 `display_name`，handle 使用低歧义 ASCII：

```text
canonical grammar: ^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$
length: 3..30
comparison: ASCII lowercase, exact byte equality after canonicalization
```

长度和是否允许 `_` 是产品选择，上述值是项目建议而非行业规范；关键是发布前固定并在数据库、API、前端和文档中共用同一个 canonicalizer。

具体合同：

1. 输入先做去首尾空白和 ASCII 小写，再校验语法；数据库只存 canonical value。
2. 唯一索引建在 canonical handle 上，不能分别依赖不同数据库 collation。
3. 保留词检查必须在 canonicalization 后执行。至少保留路由/协议/安全相关名称，例如 `api`、`admin`、`root`、`system`、`support`、`security`、`login`、`logout`、`oauth`、`oidc`、`authorize`、`token`、`userinfo`、`settings`、`me`、`users`、`assets`、`media`、`static`、`health`、`metrics`、`well-known`；同时保留产品站点和官方品牌名。
4. 创建和改名使用同一个串行化事务/唯一约束；接口可先做可用性检查，但最终以数据库唯一约束为准。
5. 改名要写 append-only history：`user_id`、old/new handle、changed_at、actor、reason。限制改名频率，敏感/高关注账号提高审核等级。
6. 默认**永久不把历史 handle 分配给另一主体**。旧 `/@handle` 可：
   - 普通改名时重定向到新 handle；
   - 隐私/安全改名时只返回 tombstone/404，不暴露新 handle；
   - 删号时返回 404/410，但仍占用历史 handle，防止冒充。
7. 若产品坚持回收，至少设置明确隔离期、禁止旧 URL/mention 自动指向新主体，并在 API 中依赖稳定 `id`。GitHub 的官方说明证明回收会打断重定向和旧引用，不能把它当作无成本策略。

如果未来必须开放 Unicode handle，应完整采用 PRECIS UsernameCaseMapped 或明确等价 profile、NFC、Unicode casefold、Bidi 检查和 UTS #39 restriction/confusable 策略；还要锁定 Unicode 数据版本并设计升级迁移。不要在 ASCII 规则上临时放宽正则。

`display_name` 则应允许 Unicode、空格和重复，只做长度、控制字符、危险双向控制符及内容治理检查；它不参与登录、URL、唯一性或授权。

## 6. ID 长度、熵与可枚举性

### 6.1 一手事实

- UUIDv4 有 122 个随机位；UUIDv7 含显式毫秒时间戳，其余部分提供随机性或单调性。[RFC 9562 §5.4](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.4)、[RFC 9562 §5.7](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.7)
- Discord Snowflake 是最多 64 位的数字，结构中包含 42 位时间戳、worker、process 和递增计数；官方文档甚至展示了如何从 ID 还原时间并生成分页边界。因此它唯一、紧凑、可排序，但并不不透明，也不是抗枚举设计。[Discord API Reference: Snowflakes](https://docs.discord.com/developers/reference#snowflakes)
- RFC 9562 建议需要难猜 UUID 的实现使用 CSPRNG，但“难猜 ID”不等于访问控制；授权仍必须独立验证。[RFC 9562 §6.9](https://www.rfc-editor.org/rfc/rfc9562.html#section-6.9)

### 6.2 推断与建议

| 候选 | 长度/信息 | 优点 | 不足 | 推荐角色 |
| --- | --- | --- | --- | --- |
| UUIDv7 | 128 位，文本通常 36 字符；暴露毫秒时间 | 分布式、可排序、数据库局部性好 | 对外偏长；可推断创建时间 | 内部 `users.id` |
| UUIDv4 | 128 位，122 随机位；文本 36 字符 | 标准、随机、成熟库多 | 文本长，索引局部性差 | 可接受的内部或公开 ID，但不是最佳展示格式 |
| 128 位随机 + base64url | 22 字符无填充；可加 `usr_` 前缀 | 短、不暴露时间、难猜、URL 安全 | 大小写敏感；需自定义类型/编码校验 | 公开 `userKey` 首选 |
| Snowflake/自增 bigint | 64 位或更短；顺序/时间明显 | 紧凑、排序和分页方便 | 易枚举、暴露规模/时间、跨节点生成复杂 | 仅在明确接受信息泄露时使用 |
| handle/email | 低熵、用户可控、会变化 | 易记 | 可枚举、可复用、存在隐私与冒充风险 | 只做查找别名，不做身份键 |

公开 key 建议：

- 16 字节由操作系统 CSPRNG 生成，base64url 无 `=`；数据库 unique，极小概率冲突时重新生成。
- 对外示例 `usr_7Yq4fP9m2KxVd3Nw8Rc1Ag`；OpenAPI 将格式写成 opaque string，预留至少 64 字符上限。
- 不将 tenant、分片、时间、地域、用户类型编码进 key。类型前缀只用于防串类型，不允许客户端据此授权。
- 列表、搜索、登录、找回等端点仍须限速和防账号枚举；不可预测 ID 只是降低批量猜测便利度，不是安全边界。

## 7. 删除、合并与外部 identity binding

### 7.1 外部身份绑定

OIDC 只保证 `(iss, sub)` 稳定；email、username、name 均不能作为外部身份主键。[OIDC Core §5.7](https://openid.net/specs/openid-connect-core-1_0-final.html#ClaimStability)

主流身份系统也把“本地账号”与“外部登录身份”分开：

- Keycloak 对通过外部 IdP 登录的用户在本地 realm 数据库建立记录，再把 broker identity 链接到本地用户。[Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/)
- Auth0 默认把不同 provider 登录视为不同用户，只有显式 account linking 才合并；官方要求链接前认证两个账号，以防攻击者靠同 email 劫持账号。[Auth0 User Account Linking](https://auth0.com/docs/manage-users/user-accounts/user-account-linking)
- ZITADEL 描述的是“一个 ZITADEL user account 链接多个 external identities”，并保留统一授权和审计轨迹。[ZITADEL Account linking](https://zitadel.com/docs/concepts/features/account-linking)

项目建议：

```text
external_identities
  id                  internal binding id
  user_id             local immutable user FK
  provider_connection explicit configured IdP connection
  issuer              exact verified OIDC issuer string
  subject             exact case-sensitive sub
  created_at
  last_authenticated_at
  profile_snapshot    optional, never authoritative for identity

UNIQUE (issuer, subject)
```

- `issuer` 必须使用 discovery/token 验证后的精确值，不能自行去斜杠、改大小写或只取 host；OIDC 明确要求 issuer 精确匹配，并指出 URI path 是 issuer 身份的一部分。[OIDC Core §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0-final.html#IDTokenValidation)、[OIDC Core §16.15](https://openid.net/specs/openid-connect-core-1_0-final.html#IssuerIdentifier)
- 对非 OIDC provider，使用 `provider_connection + provider_subject` 的等价复合键。绝不能只按 email 自动合并。
- email 相同最多触发“建议链接”；用户必须对两个账号完成近期认证，或走有审计的管理员恢复流程。

### 7.2 账号合并

Auth0 linking 的具体语义很有参考价值：指定 primary 与 secondary；合并后主 profile 和 `user_id` 保留 primary 的值，secondary identity 被挂入 primary，secondary 用户记录从用户列表移除；metadata 不自动合并，需要业务显式决定。[Auth0 User Account Linking](https://auth0.com/docs/manage-users/user-accounts/user-account-linking)

项目建议：

1. 合并必须选定 survivor `user_id`，该内部 ID、public key 和所有 public/pairwise subjects 均保持不变。
2. loser 的 credentials/external bindings 迁到 survivor 前逐项检查唯一冲突；资料、组织成员、订单、内容所有权等由各领域显式制定合并规则，禁止通用“最后写入覆盖”。
3. 保留 `user_merges(loser_id, survivor_id, merged_at, actor, reason)` tombstone，所有内部解析 loser ID 的路径只可指向 survivor，不能把 loser ID 再发给新用户。
4. 合并后撤销两边 session、refresh token、recovery code，并要求重新登录；记录不可变审计事件。
5. loser 的历史 handle/public link 应 tombstone 或受控重定向，不能释放给第三人。

### 7.3 删除与重新注册

- SCIM 允许服务端把 DELETE 实现为逻辑删除，但删除后所有读取必须 404 且列表不得再出现；与此同时，SCIM `id` 的合同仍是稳定、不可重新分配。[RFC 7644 §3.6](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.6)、[RFC 7643 §3.1](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1)
- GitHub 删除账号后 username 最终可能重新可用，且官方列出大量不能转移或会失去关联的数据，说明“删除后按用户名复活原身份”并不成立。[GitHub Personal account reference](https://docs.github.com/en/account-and-profile/reference/personal-account-reference#side-effects-of-account-deletion)

项目建议采用状态机而不是直接把行抹掉：

```text
active -> suspended
active -> deletion_pending -> deleted
deletion_pending -> active       # 仅恢复窗口内、重新认证后
```

- `deleted` 后立即撤销 session/token/credential，停止对外资料和查询，按数据保留政策擦除或匿名化 PII。
- 永久保留最小非 PII tombstone：内部 ID、public key、OIDC subject 占用、历史 handle 占用、删除时间和必要审计摘要，保证这些身份键不会授予另一个自然人。具体保留内容需再受隐私法规与数据保留政策审查。
- 重新使用同一 email 不得自动恢复旧主体；它可以创建新用户。若同一外部 `(iss, sub)` 在恢复期重现，应进入显式恢复/安全审核，而不是静默绑定新主体。
- 用户删除与外部 IdP unlink 是两种动作。unlink 只移除一种登录方式；删除才终止本地主体。Auth0 官方也把 unlink 后产生独立 secondary account 和真正删除 secondary identity 区分处理。[Auth0 Unlink User Accounts](https://auth0.com/docs/manage-users/user-accounts/user-account-linking/unlink-user-accounts)

## 8. API 版本与“尚未发布的旧接口”

### 8.1 一手事实

- Google AIP-185 要求 REST API 的 major version 位于 URI path 第一段，只暴露 `v1`、`v2`，不暴露 minor/patch；不兼容变化进入新 major，多个 major 应在合理迁移期内并存。[AIP-185: API Versioning](https://google.aip.dev/185)
- AIP-180 把 API 视为用户合同，同一 major 内不能删除/重命名字段或操作，也不能改变现有 ID/资源名的构造格式；但它同时说明，只有受控调用方且能强制升级的 API 可以按自己的兼容要求处理。[AIP-180: Backwards compatibility](https://google.aip.dev/180)
- AIP-181 规定 alpha 使用者必须预期 breaking changes，且 alpha 可快速迭代；stable 才要求 major 生命周期内无破坏性变化。[AIP-181: Stability levels](https://google.aip.dev/181)
- GitHub 把 breaking changes 放入新的日期版本，而 additive changes 可加入所有受支持版本；这同样证明“加字段”和“改语义/删字段”应区别处理。[GitHub REST API breaking changes](https://docs.github.com/en/rest/about-the-rest-api/breaking-changes)

### 8.2 项目建议

1. **项目当前尚未发布、没有外部调用方依赖时**：直接删除不满意的旧 user 路径、旧 ID 格式和 fallback；数据库测试数据一次迁移。不要为从未形成合同的接口保留 redirect、deprecated handler、双写或 UUID/新 key 双读。
2. **新的管理和业务 API 从一开始就是 `/api/v1/...`**。这样未来确有破坏性变化时可新增 `/api/v2/...`；v1 与 v2 操作同一稳定 user resource/public key，而不是分别制造两套用户。
3. v1 内允许 additive change：增加可选请求字段、新响应字段、新 endpoint；删除字段、改变 ID 格式、改变 null/默认语义、把 handle 路由改成 ID 路由都属于 breaking change。
4. Web 人类页面 `/@handle`、永久链接 `/u/{id}` 不需要 API version；媒体/静态 URL 同理。
5. OIDC 的 `/.well-known/openid-configuration`、issuer、`/authorize`、`/token`、`/userinfo` 是协议合同，不应机械改成 `/api/v1`。管理 API 可以版本化；若 OIDC issuer 或 subject 语义必须破坏性变化，应按身份迁移或新 issuer 处理，而不是期待普通 API version 隔离掉 RP 的账号键。
6. 发布门槛应明确记录：何时 v1 从 internal/alpha 进入 stable；在此之前可以删除重做，在此之后才产生兼容、弃用和迁移义务。

## 9. 建议在本地实现审阅中逐项验证

以下问题必须由代码、数据库 migration、OpenAPI 和运行时测试回答；市场资料本身不能替代本地证据。

### 标识层次

- `users` 当前主键是什么版本的 UUID，是否被所有 API 原样返回？
- token 的 `sub` 是数据库主键、公开 key、handle，还是独立 subject？删除重建后是否会复用？
- 是否存在稳定公开 `userKey`；所有下游服务的 FK/事件/ACL 使用哪一个值？
- API 是否把 `id` 当 number 返回，是否有 JS 精度或字符串格式假设？

### OIDC/OAuth

- issuer 是否固定、精确匹配并有迁移政策？
- 当前 subject 是 public 还是 pairwise；每个 client 的 sector 是否显式配置？
- client credentials token 的 `sub` 是否会被资源服务误判为 user ID？
- audience、token type 和 user/client principal 是否在类型上区分？

### Handle/profile

- handle 是否与 display name、email 或 login identifier 混用？
- 大小写、Unicode、规范化、唯一索引和前端校验是否完全一致？
- 是否有保留词、历史表、改名审计、频率限制和删除后的回收政策？
- 所有 `/@handle` 缓存与引用是否先解析成稳定 `userKey`？

### 外部身份与生命周期

- 外部身份唯一键是否为精确 `(issuer, sub)`，还是错误地按 email/provider username 合并？
- linking 是否要求证明两个账号；冲突 metadata 和领域数据如何处理？
- 删除是否撤销 session/token/credential，并保留不可复用 tombstone？
- 合并是否保持 survivor 的 public key 和所有已发行 subject，不会重写历史作者身份？

### API 与发布边界

- 所有管理/业务 API 是否统一进入 `/api/v1`？是否仍有未发布旧路径、重定向、fallback 或双读？
- OpenAPI 是否把公开 ID 标成 opaque string 并预留合理长度？
- v1 内是否存在改变 ID 格式或资源名语义的计划？
- 是否有合同测试覆盖：旧路径 404、内部 UUID 不泄露、handle 改名后 API ID 不变、同 email 不自动合并、pairwise client 得到不同 subject、删除后 ID 不复用？

## 10. 最终建议优先级

### 发布前必须决定

1. `users.id`、`public_key`、OIDC `sub`、handle、display name 五层合同及字段归属。
2. 第一方 sector 和第三方 pairwise 策略；issuer 与 subject 永不复用政策。
3. 公共 key 的格式与最大长度；内部 UUID 是否彻底禁止出现在 API。
4. handle canonical grammar、保留词、改名历史和不回收策略。
5. external identity 的 `(issuer, sub)` 唯一约束，以及 linking 必须双边认证。
6. 删除、恢复、合并的状态与 tombstone 语义。
7. 只保留 `/api/v1` 新合同，删除所有未发布旧接口和 fallback。

### 可以后续迭代

1. Unicode handle；第一版先用 ASCII handle + Unicode display name。
2. 第三方开发者自助注册和更复杂的 sector 管理。
3. 自动化账号合并建议；第一版只做用户主动、双边重新认证的 linking。
4. v2；只有出现真实破坏性需求时再引入，不提前制造空版本。

综合规范与平台证据，最稳妥的发布前重构不是“把 UUID 换成另一个漂亮 ID”这么简单，而是把**存储身份、公开资源身份、协议身份、人类别名和展示资料**真正拆开。这样内部可继续用 UUIDv7 获得工程收益，对外可以拥有简洁正规的 user key，OIDC 又能独立选择 public/pairwise 隐私边界，handle 改名也不会动摇跨服务数据关系。
