# 账户应用

- 生命周期：活跃的 Identity 配套应用
- 权威来源：本目录源码、Identity HTTP/OIDC 合同和独立 pnpm lock
- 消费者：全部站点用户和平台运营人员
- 验证：在本目录执行 `pnpm install --frozen-lockfile`，再执行 `pnpm test && pnpm typecheck && pnpm build`

Account 提供注册、登录恢复、个人资料与凭据设置、会话管理，以及 Identity 和基础服务的运营界面。Identity 始终是数据与协议权威；Account 不得自行实现用户、会话或资源存储。

`app/` 负责页面与界面，`server/` 负责同源代理和 BFF seam，`test/` 负责包级测试。依赖版本由本目录
`package.json` 与 `pnpm-lock.yaml` 固定；公共 UI 只消费 Foundation 正式 Release 制品，不依赖相邻源码。

从仓库根使用 Doctor 可同时启动 Identity 与 Account；单独开发时执行
`pnpm dev --host 0.0.0.0`，并用 `NUXT_API_BASE` 指向可访问的 Identity 地址。
