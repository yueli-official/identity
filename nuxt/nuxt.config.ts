// @yueli/identity-nuxt — Identity-owned OIDC BFF layer for consumer sites.
export default defineNuxtConfig({
  runtimeConfig: {
    // Server-only secrets.
    sealSecret: "", // NUXT_SEAL_SECRET — AES key for the session cookie (required)
    oidcClientSecret: "", // NUXT_OIDC_CLIENT_SECRET — server-only; empty for public PKCE clients
    downstreamBase: "", // NUXT_DOWNSTREAM_BASE — the backend service base URL
    guestSessionTtlSeconds: 0, // NUXT_GUEST_SESSION_TTL_SECONDS — consumer-selected; Identity clamps it
    guestCookieSecure: process.env.NODE_ENV === "production",
    authCookieSecure: process.env.NODE_ENV === "production",
    guestClaimTargets: [], // consumer-owned resource endpoints; each receives an Identity-signed claim assertion
    public: {
      oidcIssuer: "http://localhost:8081", // NUXT_PUBLIC_OIDC_ISSUER
      oidcClientId: "", // NUXT_PUBLIC_OIDC_CLIENT_ID
      oidcRedirectUri: "", // NUXT_PUBLIC_OIDC_REDIRECT_URI — must match the registered client
      oidcPostLogoutRedirectUri: "", // NUXT_PUBLIC_OIDC_POST_LOGOUT_REDIRECT_URI — exact registered URI
      oidcScopes: "openid profile email roles offline_access",
      operatorSubs: "", // NUXT_PUBLIC_OPERATOR_SUBS — UI hint only; APIs remain authoritative
      accountUrl: "http://localhost:3000",
    },
  },
});
