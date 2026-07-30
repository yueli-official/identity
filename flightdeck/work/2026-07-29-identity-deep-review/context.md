# Context

## 范围

- 本轮只修改独立 `identity` 仓库，不迁移 Commerce、Asset、Notification、Shop 或 Gallery。
- 不保留废弃协议、旧配置和旧接口兼容；正式 OAuth/OIDC 消费者合同只有在安全或规范证据明确时才改变。
- Identity 继续拥有用户、凭据、会话、授权码/token、客户端、角色、MFA/Passkey、PAT 和隐私状态。
- 业务消费者只依赖标准身份协议和稳定授权语义，不应了解 Identity 内部生产者配置。
- 仓库开始时已有未提交改动；必须区分、保护并审阅，禁止通过 reset 或 checkout 丢弃。

## 审阅准则

- 优先修复账户接管、token 泄漏、权限提升、重放、客户端混淆和隐私状态错误。
- OAuth/OIDC 行为以当前一手规范和成熟安全库为依据。
- 深模块在小接口后隐藏协议、密码学、存储和重试语义；测试通过同一接口验证。
- 不为减少代码行数重写成熟密码学或认证库。

## 验收

- Go、Nuxt 和 Account 的适用测试、构建、静态检查及漏洞扫描有明确证据。
- OAuth/OIDC、会话、MFA/Passkey 和管理权限的关键失败路径有行为测试。
- 标准 Compose 支持自带依赖和复用已有基础设施，不依赖 Doctor。
- Flightdeck 与最终仓库现实一致。
