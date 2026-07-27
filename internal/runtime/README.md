# Identity 运行时适配

本目录拥有 Identity 对通用 Foundation primitive 的服务级组合，不包含身份领域规则。

- `health.go`：组合 PostgreSQL/Redis readiness 与 Foundation health runner；
- `openapi.go`：处理显式 OpenAPI 导出；
- `telemetry.go`：读取 Identity 运行环境并组装 Foundation telemetry provider 与 HTTP client。

这些薄适配留在 Identity 仓，避免服务依赖 Platform `gokit`，也避免把 Identity 的环境变量和进程策略反向塞进
Foundation。验证使用 Identity 聚焦 package 测试，不在这里启动数据库、Redis 或 HTTP 服务。
