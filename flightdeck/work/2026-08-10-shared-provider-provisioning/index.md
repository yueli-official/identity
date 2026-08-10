# 共享 Provider 消费者 Provisioning

## Goal

为本地共享 Identity Provider 提供 Identity 自己拥有的幂等 Provisioning seam，使 Workspace 能为多个消费者逐个
准备 OIDC site/service client，而不重写 Provider 的运行配置、不重启 Provider，也不把 client secret 写入运行注册表。

## Status

Finished

## Current

现有 devseed 的解析、校验、账户与 client SQL 已深化为 `internal/devprovisioning`，完整 fixture 与新的 clients-only
`cmd/devprovision` 复用同一事务实现。clients-only 输入拒绝 account 与空声明；client id 跨类型唯一，redirect URI、
service secret/ref/audience/scope 统一校验。对账只接管历史 `doctor` 或新的 `workspace` 本地 client，重复 service
secret 复用原 bcrypt hash，不回显 secret。

正式 `go.mod` 已对齐 Foundation Go `v0.2.1`；devseed、devprovision、devprovisioning 与 publisher 的精确测试和 vet
通过。真实 PostgreSQL integration 已针对 `workspace_identity_shared` 通过，验证重复执行、不改账户、secret hash
复用与拒绝 operator-owned client。解析边界同时把缺省 post-logout URI 规范化为空数组，避免可选字段以 SQL NULL
进入非空列。

Workspace 已把 clients-only 命令接成 Environment v2 Provider-owned provisioner；Nav 与 Shortlink Shared prepare
均在 Binding 前幂等收敛各自 site client，Shortlink 同时收敛 service client。输入只进入命令环境且 registry/输出不含
secret，成功后才登记 Consumer Lease。两个 Consumer 已完成真实 OIDC 并同时绑定同一个 Provider Session。

## Next

None

## Current execution

None — Work finished. Provider crash/concurrent-up 属于 Workspace 共享运行时后续验收。

## Progress

- 2026-08-10：从 `cmd/devseed` 提取 `internal/devprovisioning`，保留完整 fixture 行为并新增 clients-only 命令；
  account 输入、空声明、未知字段、重复 client、危险 redirect 和非本地所有权 client 均 fail closed。
- 2026-08-10：精确单测与 integration skip 门禁通过；Workspace 已新增不含 Consumer clients 的独立 Identity
  Provider Environment，尚未启动或修改数据库。
- 2026-08-10：Workspace 已接入 shared Provisioning Requirement；Identity 全量 Go test/vet 通过。首次真实 prepare
  在 PostgreSQL 连接前 fail closed，未建库或执行 provisioning。
- 2026-08-10：Nav Shared prepare 连续两次真实 provision `nav-yueli-web`；PostgreSQL integration、全量 test/vet、
  operator-owned client 保护和活动 Lease 阻止 Provider Down 均通过。另修复缺省 post-logout URI 的空数组规范化。
- 2026-08-10：Shortlink Shared 幂等 provision site/service client；Shortlink 与 Nav 两个真实 OIDC、同 Provider 双 Lease、
  双/单 Lease 停止门禁及 Nav Down 后 Shortlink 继续可用均通过，Identity provisioning Work 完成。

## References

- [稳定约束](context.md)
- [执行计划](plan.md)
- [Workspace 共享运行时总控](../../../../workspace/flightdeck/work/2026-08-09-shared-development-runtime/index.md)
