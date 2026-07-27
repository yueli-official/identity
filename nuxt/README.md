# @yueli/identity-nuxt

Identity 所有的 Nuxt OIDC BFF 层，供产品站点接入统一登录、服务端会话、访客会话延续和账户入口。
产品只配置自己的 OIDC client、回调地址与下游 API，不保存访问令牌，也不自行实现授权码流程。

## 边界

- 本包拥有 PKCE、state/nonce、ID Token 验签、密封会话、刷新去重和 Identity guest/profile 协议。
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

## 验证

```text
pnpm --filter @yueli/identity-nuxt test
```
