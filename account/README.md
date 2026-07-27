# 账户应用

- 生命周期：活跃的 Identity 配套应用
- 权威来源：Catalog `platformServices.identity.companionApp`
- 消费者：全部站点用户和平台运营人员
- 验证：`pnpm --filter account test && pnpm --filter account typecheck && pnpm --filter account build`

Account 提供注册、登录恢复、个人资料与凭据设置、会话管理，以及 Identity 和基础服务的运营界面。Identity 始终是数据与协议权威；Account 不得自行实现用户、会话或资源存储。

`app/` 负责页面与界面，`server/` 负责同源代理和 BFF seam，`test/` 负责包级测试。通过 `platformctl dev up` 随任一依赖 Identity 的站点启动，使签发方、回调和服务地址均来自 Catalog。
