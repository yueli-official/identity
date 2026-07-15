# @platform/auth

- Lifecycle: active Nuxt authentication layer
- Consumers: Account-adjacent and product web apps
- Verify: `pnpm --filter @platform/auth test && pnpm typecheck`

This layer owns the consumer BFF OIDC flow, sealed `rs_session`, refresh/reauth
behavior, auth middleware/composables and shared account controls. Identity is
the protocol/account authority; consumers configure this layer rather than
implementing their own callback/session stack.
