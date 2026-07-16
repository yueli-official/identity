# @platform/auth

- 生命周期：活跃的 Nuxt 认证 layer
- 消费者：账户相关应用与产品 Web 应用
- 验证：`pnpm --filter @platform/auth test && pnpm typecheck`

该 layer 负责消费站 BFF OIDC 流程、密封的 `rs_session`、刷新与重新认证行为、认证中间件/composable 和共享账户控件。Identity 是协议与账户权威；消费者应配置本 layer，不得自行实现回调和会话栈。
