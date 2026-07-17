# Identity

Identity 是平台的身份与凭证颁发上下文。它管理 User、Guest Subject、登录会话、Guest Session、OIDC client、签名密钥和令牌签发；产品服务管理自己的业务资源与授权策略。

## Language

**User（用户）**:
已经完成账户登录、拥有稳定 Identity ID 的人类主体。
_Avoid_: Guest、浏览器、OIDC Client

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

**Device Risk Signal（设备风险信号）**:
可选的指纹、IP、UA 或行为特征，只参与滥用判断。
_Avoid_: 身份、所有权键、认领证明

## Invariants

- Identity 颁发凭证但不决定 Gallery 等产品的审核、公开或业务生命周期。
- Guest Session TTL 与 access/claim token TTL 独立；长 session 只能换短 token。
- Guest handle 是秘密且只存 hash；Guest Subject ID 不是秘密，但不能单独证明所有权。
- client 必须已注册，token audience 必须属于该 client 的允许列表。
- 同一 Guest Session 只能认领给一个 User；同 User 重试幂等，其他 User 冲突。
- 浏览器指纹永远不能替代 Guest Session。
