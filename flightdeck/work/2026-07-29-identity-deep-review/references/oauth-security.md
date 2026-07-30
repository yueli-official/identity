# Identity authentication and deployment research

Research date: 2026-07-29

## Primary sources

- [RFC 9700 — Best Current Practice for OAuth 2.0 Security](https://www.rfc-editor.org/rfc/rfc9700.html)
- [RFC 8252 — OAuth 2.0 for Native Apps](https://www.rfc-editor.org/rfc/rfc8252.html)
- [OpenID Connect Core 1.0, errata set 2](https://openid.net/specs/openid-connect-core-1_0-errata2.html)
- [ORY Fosite](https://github.com/ory/fosite)
- [go-webauthn/webauthn](https://github.com/go-webauthn/webauthn)
- [Docker Compose startup order](https://docs.docker.com/compose/how-tos/startup-order/)
- [Docker Compose merge/reset/override](https://docs.docker.com/reference/compose-file/merge/)
- [golang-migrate](https://github.com/golang-migrate/migrate)

## Decisions

1. Retain Fosite as the OAuth2/OIDC protocol engine. Identity should own policy,
   client registration, persistence and account lifecycle checks, while Fosite
   continues to own protocol parsing and grant machinery. Reimplementing the
   protocol locally would enlarge the security surface.
2. Retain `go-webauthn/webauthn` behind `authentication.PasskeyVerifier`.
   Identity owns durable, one-time ceremony state and atomic credential/session
   persistence; the library owns WebAuthn parsing and cryptographic validation.
3. Public authorization-code clients require PKCE S256. Refresh tokens remain
   durable, rotate on use and revoke the family on replay. Every refresh grant
   revalidates the subject's active lifecycle, regardless of optional scopes.
4. Redirect URIs remain exact registered values. HTTPS is required except
   loopback localhost. `oidc.allowPrivateHttpRedirects` is an explicit,
   default-off local-device extension for literal private IP addresses; RFC
   8252 standardizes loopback HTTP, not arbitrary RFC1918 HTTP host callbacks.
5. Client JWT assertions are not an Identity client contract. The Fosite
   storage methods therefore reject all assertion JTIs instead of silently
   accepting and forgetting them.
6. Account status is an authorization input at every long-lived credential
   boundary: Identity sessions, refresh grants, UserInfo, PAT verification,
   OAuth binding callbacks and pending Passkey registration.
7. Compose uses dependency health checks and
   `service_completed_successfully` for migration/key initialization.
   `compose.external.yaml` uses official `!override`/`!reset` semantics to
   remove bundled PostgreSQL/Redis dependencies while keeping the same app
   contract.

## Rejected alternatives

- A custom OAuth/OIDC implementation: no security or module-depth advantage.
- Fixed PAT HMAC fallback in production: removed; the required global secret is
  used when no dedicated PAT root is configured.
- `doctor.yaml` as a private orchestration DSL: deleted in favor of standard
  Compose and normal repository configuration.
- HTTP callbacks for public hosts or hostnames that merely resolve to private
  addresses: rejected. The local extension accepts only literal private IPs,
  avoiding DNS rebinding and environment-dependent resolution.
