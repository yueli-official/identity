# @yueli/identity-nuxt

Identity 所有的 Nuxt OIDC BFF 层，供产品站点接入统一登录、服务端会话、访客会话延续和账户入口。
产品只配置自己的 OIDC client、回调地址与下游 API，不保存访问令牌，也不自行实现授权码流程。

## 边界

- 本包拥有 PKCE、state/nonce、ID Token 验签、密封会话、刷新去重和 Identity guest/profile 协议。
- 产品 Session 只密封 Access JWT、Refresh Token、到期时间与有界身份回退；头像、角色等可变资料不重复进入 Cookie。
  Cookie 名和值的硬预算为 3500 字节，超限会在发送前失败，而不是让浏览器静默丢弃。
- 产品 Session `ys_<instance>_<digest>` 与 OIDC transaction `yt_<instance>_<digest>` Cookie 会从精确的 OIDC Client ID
  自动派生“安全可读部署标签 + 短摘要”；部署标签会去掉无助于浏览器诊断的尾部 `-web`。同主机
  上的两个站点实例（包括两个 Blog）不会互相覆盖，浏览器诊断时仍能辨认 Client；消费者不得手工命名或复制派生算法。
- 下游 `/api/v1` 转发使用 `@yueli/nuxt-runtime` 的有界 BFF，只允许受控 method、header、body 和私有 target。
- 产品仍拥有领域授权、管理页面、品牌文案和运行配置；`operatorSubs` 只用于界面提示，API 必须独立鉴权。

## 使用

在 Nuxt app 中继承本层，并安装 `@nuxt/ui`、`@yueli/ui` 与 `@yueli/nuxt-runtime`：

```ts
export default defineNuxtConfig({
  extends: ["@yueli/identity-nuxt"],
});
```

至少配置 `NUXT_SEAL_SECRET`、`NUXT_PUBLIC_OIDC_ISSUER`、`NUXT_PUBLIC_OIDC_CLIENT_ID`、
`NUXT_PUBLIC_OIDC_REDIRECT_URI`、`NUXT_PUBLIC_OIDC_POST_LOGOUT_REDIRECT_URI` 和
`NUXT_DOWNSTREAM_BASE`。生产环境的密封密钥必须稳定且不少于 32 字节。
轮换密封密钥时可临时设置 `NUXT_SEAL_SECRET_PREVIOUS`；旧 Cookie 会被读取，并在下一次访问或 Token 刷新时迁移到当前密钥。

`NUXT_DOWNSTREAM_BASE` 只配置后端 origin（或 API 版本之前的私有基础路径）；Identity Nuxt BFF adapter 独占追加
`/api/v1`。消费者不得把同一个版本前缀再写入 base，否则会形成 `/api/v1/api/v1`。

OIDC Client ID 同时是站点实例的会话命名身份，必须在一个环境内唯一且保持稳定。修改 Client ID 会开始新的产品
Session 命名空间；中央 Identity SSO Cookie 不受影响，用户通常只经历一次静默授权往返。

## 验证

```text
pnpm --filter @yueli/identity-nuxt test
```
