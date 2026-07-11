# identity-service

User-center IdP backend — milestone 2 scope: identity management, email/password
login, Redis-backed sessions, and login rate-limiting / lockout.

## Purpose

Provides user registration, authentication, and session management for the
nuxtblog platform. Stores identities and credentials in PostgreSQL; sessions and
throttle counters live exclusively in Redis (no PG writes on hot paths).

## Environment variables

| Variable              | When required          | Description                                            |
|-----------------------|------------------------|--------------------------------------------------------|
| `IDENTITY_PG_LINK`    | Runtime                | PostgreSQL link string: `pgsql:user:pass@tcp(host:5432)/identity` |
| `IDENTITY_REDIS_ADDR` | Runtime                | Redis address: `host:6379`                             |
| `TEST_PG_LINK`        | Integration tests only | Same format as `IDENTITY_PG_LINK`, test DB             |
| `TEST_REDIS_ADDR`     | Integration tests only | Redis address for integration tests                    |

The DB connection is structured fields in `manifest/config/config.yaml`
(host/port/user/pass/name; GoFrame's pgsql driver ignores `link` — see flightdeck
local-dev-gotchas §6). Redis address is still supplied via the `GF_*` override:

```sh
export GF_REDIS_DEFAULT_ADDRESS="$IDENTITY_REDIS_ADDR"
```

## Apply migrations

Uses golang-migrate. Run against the target database before starting the service:

```sh
migrate -path manifest/sql/migrations \
        -database "$IDENTITY_PG_LINK" up
```

## Running tests

Hermetic (no external deps):

```sh
go test ./...
```

Integration tests (requires PostgreSQL + Redis with migrations applied):

```sh
export TEST_PG_LINK="pgsql:user:pass@tcp(127.0.0.1:5432)/identity_test"
export TEST_REDIS_ADDR="127.0.0.1:6379"
go test -tags integration ./...
```

## Endpoints

| Method | Path                      | Description                        |
|--------|---------------------------|------------------------------------|
| POST   | `/api/v1/auth/register`   | Register a new identity            |
| POST   | `/api/v1/auth/login`      | Email/password login; sets cookie  |
| POST   | `/api/v1/auth/logout`     | Invalidate current session         |
| GET    | `/api/v1/session/me`      | Return the current session profile |
| GET    | `/healthz`                | Liveness probe                     |

OpenAPI spec: `GET /api.json`  
Swagger UI: `GET /swagger`

## OIDC / OAuth 2.0 (milestone ③)

The service exposes a standards-compliant OAuth 2.0 authorization server (backed
by [ory/fosite](https://github.com/ory/fosite)) with PKCE-enforced authorization
code flow and RS256-signed JWTs.

### OIDC endpoints

| Method    | Path                                | Description                              |
|-----------|-------------------------------------|------------------------------------------|
| GET       | `/.well-known/openid-configuration` | Discovery document (RFC 8414)            |
| GET       | `/oauth2/jwks.json`                 | Public keys in JWKS format (RFC 7517)    |
| GET       | `/oauth2/authorize`                 | Authorization endpoint (RFC 6749 §3.1)  |
| POST      | `/oauth2/token`                     | Token endpoint (RFC 6749 §3.2)          |
| GET/POST  | `/oauth2/userinfo`                  | UserInfo endpoint (OIDC Core §5.3)      |

> These endpoints write raw RFC-compliant responses and are **not** wrapped in the
> platform JSON envelope middleware (which applies only to the business API group).

### Seeded development client

Migration `0002` seeds a public PKCE client for low-level OIDC tests:

| Field           | Value                         |
|-----------------|-------------------------------|
| `client_id`     | `demo-web`                    |
| `public`        | `true`                        |
| `redirect_uris` | `http://127.0.0.1:3000/callback`, `http://localhost:3000/callback` |
| `grant_types`   | `authorization_code`          |
| `scopes`        | `openid profile email roles`  |

### Environment variables

| Variable              | Required       | Description                                                       |
|-----------------------|----------------|-------------------------------------------------------------------|
| `GF_OIDC_GLOBALSECRET`| Runtime        | ≥ 32-byte secret for HMAC auth-code signatures (never commit)     |
| `GF_DATABASE_DEFAULT_LINK` | Runtime   | PostgreSQL link: `pgsql:user:pass@tcp(host:5432)/identity`        |
| `GF_REDIS_DEFAULT_ADDRESS` | Runtime   | Redis address: `host:6379`                                        |
| `TEST_PG_LINK`        | Integration tests | PostgreSQL link for test DB (with migrations applied)          |
| `TEST_REDIS_ADDR`     | Integration tests | Redis address for integration tests                            |

Config keys `oidc.issuer` (default `http://localhost:8081`) and
`account.loginUrl` (default `http://localhost:3000/login`) can be overridden
via `manifest/config/config.yaml` or `GF_OIDC_ISSUER` / `GF_ACCOUNT_LOGINURL`.
`oidc.issuer` MUST equal the externally-reachable origin of this service — it is
stamped as the JWT `iss` claim and used to build every discovery URL, so a
mismatch silently breaks relying-party discovery and token validation.

### Auth code + PKCE walkthrough

1. Generate a PKCE verifier (`code_verifier`, 43–128 chars) and its SHA-256
   challenge (`code_challenge = BASE64URL(SHA256(verifier))`).
2. Redirect the browser to:
   ```
   GET /oauth2/authorize
     ?response_type=code
     &client_id=demo-web
     &redirect_uri=http://localhost:3000/callback
     &scope=openid%20profile%20email
     &state=<random>
     &code_challenge=<challenge>
     &code_challenge_method=S256
   ```
3. The endpoint reads the `id_session` cookie set by milestone ② login. If absent
   the user is redirected to `account.loginUrl`.
4. On success, the browser receives a `302` to `redirect_uri?code=<auth_code>&state=<state>`.
5. Exchange the code:
   ```sh
   curl -X POST http://localhost:8081/oauth2/token \
     -d grant_type=authorization_code \
     -d code=<auth_code> \
     -d redirect_uri=http://localhost:3000/cb \
     -d client_id=demo-web \
     -d code_verifier=<verifier>
   ```
   Returns `{ "access_token": "...", "id_token": "...", "token_type": "Bearer", ... }`.
6. Fetch user claims:
   ```sh
   curl -H "Authorization: Bearer <access_token>" http://localhost:8081/oauth2/userinfo
   ```

### Local stack

Use `cmd/devkit` from the platform repo root for local site development. It
starts the real PG+Redis identity service, provisions the shared test account,
and registers each site's OIDC client from the resolved local catalog:

```sh
go run ./cmd/devkit up --stack catalog/overlays/local.yaml --root . docs-ae
```

The shared local login is `test@example.com` / `Test12345`.

### Milestone ③ note — transient OIDC sessions (superseded by ④)

~~Authorization codes, PKCE state, access-token sessions, and OIDC sessions are
held **in memory** (single-instance) by the fosite `MemoryStore`. Restarting the
service invalidates all outstanding sessions. Milestone ④ will persist these to
PostgreSQL / Redis for HA and horizontal scaling.~~

See milestone ④ below.

---

## OIDC / OAuth 2.0（milestone ④）— 持久化 + Refresh Token + 撤销

### 新增端点

| 方法      | 路径                    | 说明                                            |
|-----------|-------------------------|-------------------------------------------------|
| POST      | `/oauth2/revoke`        | Token 撤销（RFC 7009）                          |
| GET/POST  | `/oauth2/end_session`   | RP 发起的登出（OIDC Core §5；MVP 版本）         |

### `offline_access` scope 与 Refresh Token 轮换

请求 `scope=offline_access` 时，token 端点随 access token 一并返回 refresh token（有效期默认 720h，可通过 `oidc.refreshTtl` 配置）。

**轮换策略（family / replay 检测）：**

- 每次用 refresh token 换新 token 时，旧 refresh token 作废，发放一个新的。
- 若旧 refresh token（已轮换）被再次使用（重放攻击），整个 family（本次会话的全部 refresh token）立即撤销，强制重新认证。

### `/oauth2/revoke`（RFC 7009）

接受 `token` 参数（refresh token 或 access token）。

- Refresh token：从 PG 删除，同 family 的所有未过期 refresh token 一并作废。
- Access token：当前为幂等空操作（access token 是无状态 JWT，到期自然失效；即时撤销可通过 denylist 实现，计划在后续 milestone 中追加）。

### `/oauth2/end_session`

1. 清除 IdP 侧的身份会话（`id_session` cookie）。
2. 撤销该会话关联的所有 refresh token（`RevokeRefreshBySession`）。
3. 当请求同时携带 `client_id` 和 `post_logout_redirect_uri`，且 URI 与该 client 注册的 `post_logout_redirect_uris` 之一完全相同时，302 回跳；缺参或未登记 URI 只返登出结果，避免开放重定向。

### 被动登出语义

业务层的 `Logout`（单会话登出）和 `LogoutAll`（撤销该 identity 全部会话）现在也会级联撤销关联的 OIDC refresh token：

- `Logout` → `RevokeRefreshBySession(sessionID)`：仅吊销当前会话的 refresh token。
- `LogoutAll` → `RevokeRefreshByIdentity(identityID)`：吊销该用户名下全部 refresh token。

这一钩子通过 `svc.SetRefreshRevoker(oidcStore)` 注入（`logic.RefreshRevoker` 接口），无需改动业务层核心逻辑。

### PG 持久化

| 表                     | 用途                                              |
|------------------------|---------------------------------------------------|
| `oidc_oauth_requests`  | 存储 OAuth 请求（授权码、PKCE state、OIDC session）|
| `oidc_refresh_tokens`  | 存储 refresh token（含 family 关联与撤销标志）    |

Access token **不**存入数据库——它是无状态 JWT，由 `iss`/`exp`/`sub`/`scope` 自携带所有信息。服务重启后 refresh token 仍然有效；access token 继续生效直至过期。

### 配置

```yaml
oidc:
  issuer: "http://localhost:8081"
  refreshTtl: "720h"      # 30 天；access/id token 固定 10 分钟（代码设置）
  # globalSecret 通过环境变量 GF_OIDC_GLOBALSECRET 传入（≥32 字节）
```

`oidc.refreshTtl` 默认值为 `720h`（30 天），可在 `manifest/config/config.yaml` 或通过 `GF_OIDC_REFRESHTTL` 环境变量覆盖。

### Seeded client（milestone ④ 更新）

Migration `0003` 为 `demo-web` 客户端追加了 `offline_access` scope 以及 `refresh_token` grant type：

| Field           | Value                                                        |
|-----------------|--------------------------------------------------------------|
| `client_id`     | `demo-web`                                                   |
| `grant_types`   | `authorization_code`, `refresh_token`                        |
| `scopes`        | `openid profile email roles offline_access`                  |
