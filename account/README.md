# Account

- Lifecycle: active Identity companion app
- Authority: Catalog `platformServices.identity.companionApp`
- Consumers: all site users and platform operators
- Verify: `pnpm --filter account test && pnpm --filter account typecheck && pnpm --filter account build`

Account provides registration/login recovery, profile and credential settings,
session management, and the operator views for Identity plus proxied foundation
service administration. Identity remains the data/protocol authority; Account
must not implement its own users, sessions or asset storage.

`app/` owns pages and UI, `server/` owns same-origin proxy/BFF boundaries, and
`test/` owns package tests. Start it through `platformctl dev up` with any site
that depends on Identity so issuer, callback and service URLs come from Catalog.
