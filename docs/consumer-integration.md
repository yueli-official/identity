# Identity 消费者接入

本文说明独立网站、Web App、原生 App 和后端服务如何消费 Identity。字段与错误的最终真值是
`contracts/openapi.json`、`contracts/errors/catalog.json`、OIDC discovery 和正式发布的
`@yueli/identity-nuxt`；本文只维护接入顺序与信任规则。

跨服务总顺序见 Workspace 的 `docs/platform-consumer-integration.md`。

## 1. 选择客户端类型

| 调用方 | grant | client 类型 | secret |
| --- | --- | --- | --- |
| Nuxt SSR/BFF | Authorization Code + PKCE | public 或 confidential | 只允许 confidential BFF 在服务端保存 |
| 浏览器 SPA | Authorization Code + PKCE S256 | public | 禁止 |
| 原生/桌面 App | Authorization Code + PKCE S256 | public | 禁止内置 |
| 后端 API/worker | client credentials | confidential | secret store |

网站默认使用 Nuxt BFF。浏览器不获得平台 access token，`@yueli/identity-nuxt` 负责 PKCE、state/nonce、
ID Token 验签、密封 session 和 refresh 去重。SPA/原生模式需要产品自己承担安全存储、CORS、撤销和系统浏览器
回调；Identity 当前不发布通用原生 SDK。

## 2. 注册 OIDC client

每个环境和产品使用独立 `client_id`，至少提交：

- 类型：public PKCE 或 confidential BFF。
- 精确 `redirect_uris` 与 `post_logout_redirect_uris`；不接受通配域名。
- grant/response type；用户登录使用 authorization code。
- scopes：按需选择 `openid profile email roles offline_access`。
- 下游 audiences；一个 client 不应获得所有服务 audience。
- subject 类型/sector；消费者不得假设所有 client 的 `sub` 永远相同。

Identity 的公开 issuer 必须是稳定、外部可访问的规范 origin。容器名、局域网 IP 和临时端口可以作为内部 target，
但不能替代生产 issuer。redirect URI、cookie secure、Account origin、WebAuthn RP ID/origin 必须与公网地址一致。

## 3. Public User 合同

- 跨服务用户主键是 `userKey`，形如 `usr_` 加 22 个 URL-safe 字符。
- 内部 Identity UUID 不进入消费者数据库、URL、日志或 webhook。
- OIDC `sub` 是协议主体；启用 pairwise subject 时可按 client 变化，不能作为全平台外键。
- bearer 明确包含 `subject_kind=user|client|guest`。资源服务必须先校验 kind，再解释 subject/client_id。
- email、handle 和头像均可变化，不是用户主键。

公开用户查询：

- `GET /api/v1/users/{userKey}`
- `GET /api/v1/users?ids=...`
- `GET /api/v1/users/by-handle/{handle}`

消费者只缓存展示所需的公开资料，并按产品需求设置失效策略；不得复制 Identity 凭据或账户状态真值。

## 4. Nuxt BFF 最小配置

安装正式 release 中的 `@yueli/identity-nuxt` 及其 Foundation peer dependencies，然后继承 layer：

```ts
export default defineNuxtConfig({
  extends: ["@yueli/identity-nuxt"],
  modules: ["@nuxt/ui", "@yueli/ui", "@yueli/nuxt-runtime"],
  runtimeConfig: {
    downstreamBase: process.env.NUXT_DOWNSTREAM_BASE,
    oidcClientSecret: process.env.NUXT_OIDC_CLIENT_SECRET || "",
    sealSecret: process.env.NUXT_SEAL_SECRET,
    public: {
      oidcIssuer: process.env.NUXT_PUBLIC_OIDC_ISSUER,
      oidcClientId: process.env.NUXT_PUBLIC_OIDC_CLIENT_ID,
      oidcRedirectUri: process.env.NUXT_PUBLIC_OIDC_REDIRECT_URI,
      oidcPostLogoutRedirectUri:
        process.env.NUXT_PUBLIC_OIDC_POST_LOGOUT_REDIRECT_URI,
      accountUrl: process.env.NUXT_PUBLIC_ACCOUNT_URL,
    },
  },
});
```

`NUXT_SEAL_SECRET` 必须稳定且不少于 32 字节，只在服务端。public PKCE client 的
`NUXT_OIDC_CLIENT_SECRET` 必须为空。产品不要再实现第二套 `/auth/callback`，不要把 access/refresh token 放入
localStorage，也不要从前端 roles 决定 API 授权。

## 5. 后端验证与 machine client

资源服务通过 Identity JWKS 在本地验证短期 JWT，至少检查签名算法、`kid`、issuer、audience、expiry、
最大生命周期和 `subject_kind`。权限判断使用精确 scope/role；不能只检查“token 可解码”。

machine client：

- 只允许 confidential client 使用 client credentials。
- 每个目标服务使用受限 audience/scope；不要复用管理员 token。
- token endpoint 可以走内部网络，但 token 的 `iss` 仍是公开 issuer。
- client secret 只进入部署 secret store，不写入仓库、前端 runtime 或日志。

协议端点 `/.well-known/openid-configuration`、`/oauth2/*`、`/oauth2/jwks.json` 使用标准 OAuth/OIDC 错误；
业务 `/api/v1` 使用平台 problem envelope。调用方必须记录 request ID 和稳定错误 code，不依赖中文 message。

## 6. 验收

API/协议：

- public client 没有 secret 也能完成 code + PKCE S256；缺少/错误 verifier 必须失败。
- redirect/post-logout URI 必须精确匹配；错误 audience、issuer、过期 token 和缺失 `subject_kind` 被拒绝。
- user/client/guest token 在各自接口上 fail closed。
- Public User 查询不泄漏内部 UUID、邮箱等非公开字段。
- 单会话退出、全退出与 refresh rotation 行为符合 OpenAPI/OIDC 合同。

Web 使用 CLI Playwright：

```powershell
pnpm exec playwright test
```

至少验证登录跳转、callback、刷新后 session、退出、未登录/越权页面、移动视口和关键无障碍路径；同时断言无
未处理页面异常与关键 5xx。截图只能作为辅助，不能替代交互和网络断言。

## 7. 发布边界

消费者只依赖 release/tag 提供的 Go module、Nuxt tarball 和 OpenAPI，不提交跨仓相对 `replace` 或相邻源码。
如果 `main` 已有但 release 尚未包含 Public User/`subject_kind` 变更，生产消费者必须等待正式发布并按迁移说明
升级。破坏性 wire 变化才引入 `/api/v2`；OIDC 标准端点按协议演进，不添加 `v=1` 查询参数。
