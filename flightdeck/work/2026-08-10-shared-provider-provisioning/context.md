# Context

## 稳定边界

- Identity 拥有 OIDC client 的 schema、校验和幂等对账语义；Workspace 只编排 Provisioning Requirement 的时机、
  非敏感身份与结果引用。
- Shared Provider 是独立 Identity Target Session。Consumer Session 不拥有或重启它，只在 provisioning 成功后取得
  Runtime Binding 和 Binding Lease。
- Consumer provisioning 只改变该消费者声明的 site/service client，不创建或修改共享测试账户，也不重写 issuer、
  数据库或 Provider 进程配置。
- Provisioning 输入可以含本地 client secret，但日志、错误、Workspace registry 与可提交合同不得回显或保存 secret；
  运行注册表最多保存非敏感 requirement identity 与生成制品引用。
- 对同一声明重复执行必须收敛为同一数据库状态；不兼容的既有 client 不得被静默接管。
- 本能力只服务显式本地开发合同，不成为远程管理入口或生产 provisioning 协议。

## 当前实现事实

- `cmd/devseed` 从 `IDENTITY_DATABASE_URL` 与 `IDENTITY_DEV_SEED` 读取一个 JSON 文档，在 migration 后直接对 PostgreSQL
  执行开发 fixture 对账。
- 当前 Workspace Complete Environment 把每个站点的 client 声明放进 Identity 的 Provider prepare task；这会让
  不同消费者形成不同的 Identity Runtime Digest，无法安全共享同一个 Provider Session。
