# 用户标识与账户合同深度审阅计划

## P0 — 基线与影响面

- [x] 盘点 User、Profile、OIDC subject、Guest、外部 binding、Admin 与 Audit 的标识模型。
- [x] 扫描全部消费者对 Identity ID、`sub`、username、公开 profile 和账号生命周期的使用。
- [x] 运行 Identity、Account、Nuxt 基线验证并记录现有 OpenAPI 合同。

## P1 — 标准与主流实践调研

- [x] 调研 OIDC subject、pairwise identifier 和公共用户标识的规范要求。
- [x] 调研主流身份库对内部 ID、外部 ID、用户名、邮箱与生命周期的建模。
- [x] 调研大型网站公开用户 URL、不可变用户 ID、handle 重命名与 API 版本策略。

## P2 — 目标合同与深模块设计

- [x] 明确内部主体键、协议 subject、公开 user key、handle 和 display name 的独立职责。
- [x] 设计解析、规范化、保留词、重命名、别名、枚举与删除语义。
- [x] 评估消费者迁移、数据库变更、API 版本和不兼容删除策略。

## P3 — 高置信重构

- [x] 先补目标接口行为测试，再实现已确认的 ID/Profile/User 深模块。
- [x] 更新数据库迁移、OpenAPI、错误目录、Account 与 Nuxt 消费接口。
- [x] 迁移扫描确认的跨仓消费者，不扩散用户标识规则。

## P4 — 验收与收口

- [x] 运行 Identity Go、Account、Nuxt 及受影响消费者测试和静态检查。
- [x] 使用 CLI Playwright 验收登录、资料、公开用户合同、管理页面和媒体交付的适用流程。
- [x] 完成人工复审与 Flightdeck 收口；提交和发布等待单独授权。
