# Identity 消费者接入文档

## Goal

建立 Identity 自己拥有的消费者接入说明，使仓外网站/App 能正确选择 OIDC 模式、注册 client、使用 Public
User Key、配置 Nuxt BFF 并完成认证验收，而不需要复制现有产品内部配置。

## Status

Finished

## Current

`docs/consumer-integration.md` 已提供客户端类型、OIDC 注册、Public User、Nuxt BFF、machine client、
CLI Playwright 验收与发布边界，并由 README 建立入口。

## Next

None。

## Current execution

Completed。

## Progress

- 2026-08-02：用户要求每个基础服务都保存自己的接入文档。
- 2026-08-02：文档与实际 Nuxt runtime config、OIDC/Public User 合同核对完成；Identity mailer 通过本地
  Notification 新 client 组合测试。

## References

- [稳定边界](context.md)
