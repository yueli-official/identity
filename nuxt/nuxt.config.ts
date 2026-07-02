// @platform/auth — a Nuxt layer implementing the OIDC BFF for consumer sites.
// Consuming apps `extends` it and set their own
// client/issuer/downstream via runtimeConfig (NUXT_* env in prod).
export default defineNuxtConfig({
  runtimeConfig: {
    // Server-only secrets.
    sealSecret: '', // NUXT_SEAL_SECRET — AES key for the session cookie (required)
    downstreamBase: '', // NUXT_DOWNSTREAM_BASE — the backend service base URL
    public: {
      oidcIssuer: 'http://localhost:8081', // NUXT_PUBLIC_OIDC_ISSUER
      oidcClientId: '', // NUXT_PUBLIC_OIDC_CLIENT_ID
      oidcRedirectUri: '', // NUXT_PUBLIC_OIDC_REDIRECT_URI — must match the registered client
      oidcScopes: 'openid profile email roles offline_access'
    }
  }
})
