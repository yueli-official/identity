# Identity 深度审阅与重构计划

## P0 — 基线与认证边界审计

- [x] 记录既有 diff、代码构成、运行模块和外部依赖。
- [x] 运行 Go、Nuxt、Account 的测试、构建、静态检查和漏洞扫描。
- [x] 审计 OAuth/OIDC、会话、客户端、管理员、MFA/Passkey、PAT 与隐私状态边界。

## P1 — 规范与成熟方案判断

- [x] 对照 OAuth/OIDC、安全最佳实践和当前采用库的一手资料。
- [x] 记录保留、升级、替换或拒绝结论。

## P2 — 高置信重构

- [x] 删除废弃实现、兼容入口和重复基础设施。
- [x] 修复账户、token、权限、重放或隐私缺陷。
- [x] 深化认证模块接口并用行为测试保护。

## P3 — 部署合同

- [x] 提供标准 Docker Compose 独立部署入口。
- [x] 支持复用已有 PostgreSQL、Redis 与其他基础设施。
- [x] 校验配置、migration、健康检查和文档一致。

## P4 — 最终验收

- [x] 完整测试、独立构建、静态检查和漏洞扫描通过。
- [x] OpenAPI、错误目录、前端合同和文档一致。
- [x] 完成最终人工 review 并关闭 Flightdeck Work。

## Validation evidence

- `GOWORK=off go test ./...`
- `GOWORK=off go test -run '^$' -tags=integration ./...`
- `go vet ./...`
- `staticcheck ./...`
- `govulncheck ./...` — 0 reachable vulnerabilities
- `go run ./cmd/errorcatalog --check`
- OpenAPI export smoke using `GF_GCFG_FILE` + `GF_OIDC_GLOBALSECRET`
- Account: 53 tests, typecheck, production build
- Identity Nuxt: 30 tests, package dry-run
- Compose YAML static parse; runtime validation pending because Docker CLI is unavailable locally
