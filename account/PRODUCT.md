# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

使用月离站群账户的人，以及从 Nav 等产品查看成员公开资料的访客。账户持有人管理登录方式、安全状态和可公开展示的资料；访客需要确认一个成员是谁并继续访问其公开链接。

## Product Purpose

Account 是 Identity 唯一配套界面：承载登录、账户安全与个人资料管理，并提供统一的公开用户资料页面。成功意味着同一个人可被稳定引用，同时不把内部存储主键或机器合同当作面向用户的身份。

## Positioning

账户身份被明确拆成内部 UUIDv7、稳定 Public User Key、可修改 Handle 与显示名；站群产品不复制用户记录，也不另造第二套公开短号。

## Operating Context

用户会从站群产品中的成员链接进入 `/@handle`，也可能通过 `/u/{userKey}` 打开永久地址。登录用户在 Account 内维护显示名、Handle、简介、头像、封面、社交链接及安全凭据。

## Capabilities and Constraints

- `/@handle` 是首选公开资料地址；`/u/{userKey}` 是不可变兜底地址。
- Public User Key 固定为 8 位 Base58，同时服务于界面用户号、API、OIDC `user_key` 与跨服务引用；它不是凭据。
- 公开资料不得暴露邮箱、角色、内部 UUID、账户状态或安全信息。
- 资料不存在、已禁用或已删除时统一表现为未找到。

## Evidence on Hand

真实资料字段、会话与媒体引用来自 Identity API；现有 Account 页面与 Foundation UI 是界面实现依据。没有可展示的关注数、内容统计或认证背书，页面不得虚构。

## Product Principles

- 人可以看 Public User Key 与 Handle，服务使用同一个稳定 Public User Key。
- 公开资料保持克制，只呈现用户主动公开的字段。
- 永久地址与可发现地址并存，修改 Handle 不切断身份。
- 所有站点链接回同一个 Identity 权威，不维护影子用户资料。

## Accessibility & Inclusion

公开资料页须支持键盘导航、可见焦点、浅色与深色模式，并在桌面和移动端保持清晰阅读顺序。
