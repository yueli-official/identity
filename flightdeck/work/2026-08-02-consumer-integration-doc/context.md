# Context

- Identity 是唯一身份权威，消费者不能复制凭据、会话或内部 UUID。
- Web 首选 `@yueli/identity-nuxt` BFF；SPA/原生使用 Authorization Code + PKCE，不能携带 client secret。
- 跨服务用户引用使用 `usr_` Public User Key；OIDC `sub` 只按协议语义消费。
- 字段级真值是生成 OpenAPI、错误目录和正式发布 package；文档不复制完整 schema。
- Web 验收必须使用 CLI Playwright。
