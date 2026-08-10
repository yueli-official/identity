# Plan

## P0 — Provisioning seam

- [x] 从现有 devseed 中分离 Identity-owned client 声明、校验与幂等对账，不复制 SQL 或 client 合同。
- [x] 定义消费者专属输入：只允许 site/service clients，拒绝账户 fixture 与空/冲突声明。

## P1 — 产品门禁

- [x] 覆盖重复执行、site/service client 更新、不改账户和 secret 不回显。
- [x] Identity 精确测试、全量 Go 测试与 vet 通过。

## P2 — Workspace 接线

- [x] 独立 Identity Provider Environment 只准备共享 Provider 身份，不包含任一消费者 client。
- [x] Workspace 在 Binding 前执行声明式 Provisioning Requirement，成功后才登记 Consumer Lease。

## P3 — 真实组合

- [x] Shortlink 与 Nav 绑定同一 Identity Provider，两个 client 均可完成真实 OIDC。
- [x] 停止任一 Consumer 不影响另一个；活动 Lease 阻止停止 Provider。
