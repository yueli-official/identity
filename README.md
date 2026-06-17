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

GoFrame reads runtime secrets via its `GF_*` env-var override convention:

```sh
export GF_DATABASE_DEFAULT_LINK="$IDENTITY_PG_LINK"
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

### Seeded client

Migration `0002` seeds a `demo-web` public client (PKCE, no client secret):

| Field           | Value                         |
|-----------------|-------------------------------|
| `client_id`     | `demo-web`                    |
| `public`        | `true`                        |
| `redirect_uris` | `http://localhost:3000/cb`    |
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

### Auth code + PKCE walkthrough

1. Generate a PKCE verifier (`code_verifier`, 43–128 chars) and its SHA-256
   challenge (`code_challenge = BASE64URL(SHA256(verifier))`).
2. Redirect the browser to:
   ```
   GET /oauth2/authorize
     ?response_type=code
     &client_id=demo-web
     &redirect_uri=http://localhost:3000/cb
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

### Milestone ③ note — transient OIDC sessions

Authorization codes, PKCE state, access-token sessions, and OIDC sessions are
held **in memory** (single-instance) by the fosite `MemoryStore`. Restarting the
service invalidates all outstanding sessions. Milestone ④ will persist these to
PostgreSQL / Redis for HA and horizontal scaling.
