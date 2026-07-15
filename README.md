# Identity service

- Lifecycle: active platform service
- Authority: Catalog `platformServices.identity`, migrations and generated OpenAPI
- Consumers: Account, every product BFF/API, Asset and other platform services
- Verify: `go test ./services/identity/...`

Identity is the single identity provider for the site group. It owns accounts,
credentials, public profiles, login sessions, roles, personal access tokens,
OAuth2/OIDC clients and signing keys. Product services own their domain
permissions and content; they consume Identity subjects and scopes instead of
copying user records or implementing login.

## Runtime model

- PostgreSQL is the durable source for identities, credentials, profiles,
  Identity sessions, OIDC protocol state, refresh tokens, clients and keys.
- Redis is a session hot cache plus rate-limit/lockout dependency. A Redis miss
  can recover a valid Identity session from PostgreSQL.
- `ory/fosite` provides the OAuth2/OIDC protocol state machine. Public clients
  use authorization code + PKCE S256. Access tokens are short-lived RS256 JWTs;
  consumers validate them locally through JWKS.
- Refresh tokens are persisted and rotated. Single-session logout, logout-all
  and RP-initiated logout revoke the appropriate refresh-token scope.
- Account is the one per-environment companion UI declared by Catalog; it is
  not a second identity authority.

The durable implementation contract is documented in
`flightdeck/knowledge/auth/identity-oidc-provider-boundary.md` and
`flightdeck/knowledge/auth/login-session-durability.md`.

## API surfaces

- `/api/v1/auth/*`: registration, password login/logout, verification, password
  reset/change and optional Google login.
- `/api/v1/session/*`: current identity, profile/media updates, credentials and
  session/device management.
- `/api/v1/profiles*`: public profile lookup.
- `/api/v1/pat*`: personal access token lifecycle and verification.
- `/api/v1/admin/*`: user, capability and provider operations plus scoped
  platform-service proxies used by Account.
- `/.well-known/openid-configuration`, `/oauth2/authorize`, `/oauth2/token`,
  `/oauth2/userinfo`, `/oauth2/revoke`, `/oauth2/end_session` and
  `/oauth2/jwks.json`: raw standards endpoints without the platform envelope.
- `/healthz`, `/readyz`, `/api.json`, `/swagger`: operations and discovery.

Use generated OpenAPI as the field-level API authority.

## Directory map

- `api/v1/`: business request/response contracts.
- `cmd/identity/`: dependency composition, routes and startup validation.
- `internal/logic/`: identity/session/credential domain behavior.
- `internal/oidc/`: Fosite store, PG transaction adapter, claims and keys.
- `internal/dao/` and `internal/repo/`: durable persistence and cache seams.
- `internal/controller/`: business, OIDC and OAuth-login HTTP boundaries.
- `manifest/config/`: configuration template; real config is ignored.
- `manifest/sql/migrations/`: the only schema history authority.

## Development

Use the Catalog-driven lifecycle for normal work; it provisions OIDC clients
and the shared local account (`test@example.com` / `Test12345`) consistently:

```powershell
pnpm platformctl dev up --file catalog/overlays/local.yaml --root . docs-main
pnpm platformctl dev status --root .
```

For isolated service work, copy `manifest/config/config.example.yaml` to the
ignored `config.yaml`. Required runtime values include PostgreSQL, Redis and a
stable `GF_OIDC_GLOBALSECRET` of at least 32 bytes. `oidc.issuer` must equal the
externally reachable Identity origin.

```powershell
go run ./services/identity/cmd/identity
go test ./services/identity/...
```

Integration tests use `TEST_PG_LINK` and `TEST_REDIS_ADDR`. Never replace the PG
session/OIDC stores with memory implementations in production wiring.
