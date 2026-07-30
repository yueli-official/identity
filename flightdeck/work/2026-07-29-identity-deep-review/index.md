# Identity 深度审阅与重构

## Goal

在不兼容废弃实现的前提下，深度审阅 Identity 的 OAuth/OIDC、会话、用户凭据、MFA/Passkey、机器身份、
权限、隐私与管理能力，修复真实安全缺陷，收敛模块接口，并提供标准 Docker Compose 独立与复用部署入口。

## Status

Complete

## Current

Identity 深审与高置信重构完成。OAuth/OIDC、PAT、OAuth binding、Passkey、管理员生命周期和配置注入的
fail-open 边界已关闭；Doctor 已删除，标准 Compose 同时提供自带依赖与复用既有 PostgreSQL/Redis 模式。

## Next

Identity 深审改动已在本地 commit `f0a26a6` 收口，按用户要求暂不 push。允许发布后再推送/打版并更新
Workspace 精确锁。Docker CLI 不在当前机器，镜像启动需在具备 Docker Compose 2.24.4+ 的环境执行
README 命令验证。

## Current execution

Complete.

## Progress

- 保留并审阅开始时已有的 Doctor 删除、README 和私网 HTTP redirect 改动。
- 修复刷新 token 在角色查询失败、无 roles scope 或账户停用时继续签发的问题。
- UserInfo、PAT、OAuth 自动链接/绑定和 Passkey 注册统一检查当前 active 生命周期。
- 未支持的 client JWT assertion 明确 fail-closed；deleted 生命周期不可重新激活。
- PAT 不再使用生产固定回退，缺省派生自必填全局根密钥。
- 增加 `GF_*` 环境配置覆盖层，修复文档/Compose 环境变量此前实际不生效的问题。
- 升级 gRPC、OpenTelemetry、x/text 等依赖；可达漏洞由 5 个降为 0。
- 新增 `compose.yaml`、`compose.external.yaml`、环境模板、迁移/init 编排和 distroless readiness probe。
- Go 全量测试、integration-tag 编译、vet、staticcheck、govulncheck、错误目录检查、Account 测试/类型检查/
  build、Nuxt 测试/pack 全部通过。

## References

- [稳定范围与约束](context.md)
- [执行计划](plan.md)
- [OAuth/OIDC 与部署研究](references/oauth-security.md)
