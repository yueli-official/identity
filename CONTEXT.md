# Identity

Identity 是平台的身份与凭证颁发上下文。它管理 User、Guest Subject、登录会话、Guest Session、OIDC client、签名密钥和令牌签发；产品服务管理自己的业务资源与授权策略。

## Language

**User（用户）**:
已经完成账户登录、拥有稳定内部 User ID 与 Public User Key 的人类主体。
_Avoid_: Guest、浏览器、OIDC Client

**Internal User ID（内部用户 ID）**:
Identity 数据边界内用于关系和事务的一致、不可变主键，不作为面向用户的地址或跨服务公开合同。
_Avoid_: Public User Key、OIDC Subject、User Handle

**Public User Key（公开用户键）**:
平台 API、跨服务用户引用、界面用户号和永久公开资料地址共同使用的 8 位 Base58 稳定公开键；由 Foundation Identifier `compact-url-v1` 分配，一经分配不可变、不可复用。
_Avoid_: 数据库 UUID、第二套短号、User Handle、Display Name、授权秘密

**OIDC Subject（OIDC 主体标识）**:
由 issuer 在明确 client 或 sector 范围内分配的稳定主体值，身份只能由 `(iss, sub)` 联合确定。
_Avoid_: Public User Key、Internal User ID、邮箱、User Handle

**Public User Key Claim（公开用户键声明）**:
Identity 在用户 ID Token、用户 Access Token 和 UserInfo 中附加的 `user_key`，供平台产品建立跨服务用户引用。它不改变 `(iss, sub)` 的 OIDC 身份语义，machine client 不携带该声明。
_Avoid_: 把 `user_key` 当成通用 OIDC 主体、把 pairwise `sub` 当成平台外键

**User Handle（用户句柄）**:
规范化后唯一、可修改的人类可读别名，是 `/@handle` 公开资料地址的首选入口，不承担所有权或外键职责。
_Avoid_: username、Display Name、Public User Key

**Display Name（显示名）**:
允许重复和修改、用于界面展示的 Unicode 文本，不参与登录、解析或授权。
_Avoid_: User Handle、登录名、所有权键

**Guest Subject（游客主体）**:
Identity 为登录前业务连续性创建的稳定临时主体 ID。它可以拥有资源，但不等于注册账户。
_Avoid_: 浏览器指纹、匿名 IP、临时 token

**Guest Session（游客会话）**:
把 opaque handle、registered client、Guest Subject、有效期与认领状态关联起来的持久服务端记录。
_Avoid_: Guest JWT、浏览器 cookie、登录 session

**Guest Session Handle（游客会话句柄）**:
只交给 BFF/browser cookie 的 256-bit 随机秘密；Identity 只保存 hash。
_Avoid_: Guest Subject ID、JWT、指纹

**Guest Credential（游客凭证）**:
Identity 基于有效 Guest Session 签发的短期、单 resource audience JWT。
_Avoid_: Guest Session、长期 bearer

**Guest Claim（游客认领）**:
User 对 Guest Session 的幂等绑定，以及 Identity 向资源服务签发的短期所有权迁移断言。
_Avoid_: 由浏览器直接提交 guest ID、重新创建资源

**Requested Session TTL（请求会话期限）**:
消费站 BFF 的 server-only policy 参数。
_Avoid_: Identity 全局站点默认、浏览器可控值

**Effective Session TTL（有效会话期限）**:
Identity 校验并应用安全上限后实际批准的期限；BFF 必须用它设置 cookie。
_Avoid_: Access Token TTL

**Privacy Status Capability（隐私状态能力）**:
用户生成并自行保存的高熵秘密；Identity 只保存 digest，用于账号删除后读取同一 Rights Request。
_Avoid_: 登录会话、公开 Request ID、跨站用户画像

**Device Risk Signal（设备风险信号）**:
可选的指纹、IP、UA 或行为特征，只参与滥用判断。
_Avoid_: 身份、所有权键、认领证明

**Publisher Attestation（发布者证明）**:
Identity 对已认证 User 面向特定消费者和命名空间声明了某个精确制品摘要的长期签名事实。
_Avoid_: Access Token、上架许可、代码安全证明

**External Identity Binding（外部身份绑定）**:
Identity 向外部 Provider 验证过的稳定账号 ID 与 User 的所有权绑定事实。
_Avoid_: OAuth 登录凭据、用户名、邮箱、Profile 社交链接

**External Login Provider（外部登录提供商）**:
Identity 明确支持并由管理员配置的 OAuth/OIDC 身份来源，例如 Google 或 QQ；Provider Adapter 固定协议行为，管理员只管理实例凭据、启停和回调配置。
_Avoid_: External Identity Binding、允许任意授权端点的通用 HTTP 配置、产品站自己的 OIDC Client

**Personal Access Token（个人访问令牌）**:
User 为程序调用创建、带固定权限目录和有效期的 API Credential；明文只展示一次，可独立撤销，不建立浏览器登录会话。
_Avoid_: 登录方式、Identity Session、OIDC Access Token、Service Client Credential

**Platform Publication Proof（平台发布证明）**:
Registry 审核通过后对精确制品摘要与 Publisher Attestation 签发的上架批准事实。
_Avoid_: Publisher Attestation、Identity 凭证、远程撤销开关

## Invariants

- Identity 颁发凭证但不决定 Gallery 等产品的审核、公开或业务生命周期。
- Internal User ID 不进入公开用户合同；Public User Key 不可复用，邮箱、handle 和 display name 都不能充当所有权键。
- Public User Key 由 Foundation Identifier `compact-url-v1` 生成，固定 8 位 Base58；数据库唯一约束负责原子占用，碰撞由统一分配器重试。
- 公开资料以 `/@handle` 为首选地址，以 `/u/{userKey}` 为不可变兜底地址；不存在第二套公开用户短号。
- OIDC 身份始终以 `(iss, sub)` 判断；public 与 pairwise subject 的适用范围必须显式，不能从 User ID 或 handle 临时推导。
- 平台产品需要持久引用用户时读取已验证的 `user_key` claim；迁移期可以在确认 public subject 的第一方 client 上回退到 `sub`，但不得把该回退扩展为通用 OIDC 规则。
- User、Guest 和 machine client 的主体种类必须可区分，不能只靠一个无类型的 `sub` 字符串猜测。
- Guest Session TTL 与 access/claim token TTL 独立；长 session 只能换短 token。
- Guest handle 是秘密且只存 hash；Guest Subject ID 不是秘密，但不能单独证明所有权。
- client 必须已注册，token audience 必须属于该 client 的允许列表。
- 同一 Guest Session 只能认领给一个 User；同 User 重试幂等，其他 User 冲突。
- 浏览器指纹永远不能替代 Guest Session。
- Identity 只编排其他 Data Owner；账号删除是最终化任务，状态能力不授予任何其他账户权限。
- Publisher Attestation 只证明 User 的精确投稿声明；Registry 仍拥有 namespace、审核、上架和下架决策。
- Publisher signing key 与 OIDC、step-up 和 Registry publication key 必须按用途隔离；`kid` 只是验签 key 查找提示。
- External Identity Binding 使用 Provider 稳定账号 ID，不能由 login、email、Profile 链接或浏览器自报字段建立。
- External Login Provider 的 Client Secret 只由 Identity 控制面持有；Provider 配置不能把任意外部端点变成受信身份来源。
- External Login Provider 必须声明注册策略：只有能提供并验证 Identity 所需邮箱的 Adapter 才能创建 User；QQ 固定为 existing-user-only，只能绑定和登录既有 User。
- Personal Access Token 只授予目录中声明的最小 API scope，不能换取浏览器 Cookie，也不能接受任意自定义 scope。
- 公开 User 读取合同从 `/api/v1/users*` 开始；未发布的 `/api/v1/profiles*` 不提供兼容或 fallback。
- User Handle 规范化为小写 ASCII 3–30 位且历史值不重新分配；Display Name 不参与地址、解析、登录或授权。
- User 的 avatar/cover 只保存 Asset `mediaKey`；公开响应与 OIDC claim 不保存或信任任意外部图片 URL。
- 业务 API 使用 `/api/v1` 等版本路径；OIDC discovery、authorize、token、userinfo 等标准端点保持标准路径。
- 产品服务的用户所有权、管理员配置和用户审计 actor 使用 Public User Key；只在 Identity 内部关系中使用 UUID。

## Durable review

- User 标识分层、主流实现调研、缺陷审计、消费者迁移和验收结果统一保存在
  [2026-08-02 User 合同 Work](flightdeck/work/2026-08-02-user-identity-contract-review/index.md)。
- 后续先复读该 Work 的 55 项一手引用、当前合同审计和实施结果；只有标准、合同前提或外部产品行为实质变化时
  才重新调研。
