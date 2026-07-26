// Nuxt 4 config for the account-center app.
// @nuxt/fonts ships bundled inside @nuxt/ui v4, so it is not listed as a
// separate module here (@nuxt/ui registers it automatically).
export default defineNuxtConfig({
  modules: ["@nuxt/ui", "@yueli/ui"],
  css: ["~/assets/css/main.css"],
  devServer: {
    port: Number(process.env.NUXT_DEV_PORT || "3000"),
  },
  runtimeConfig: {
    public: {
      identityAudience:
        process.env.NUXT_PUBLIC_IDENTITY_AUDIENCE || "identity-api",
    },
    // Server-side base for backend calls during SSR. The dev proxy below only
    // covers real browser requests; Nitro's internal SSR $fetch bypasses it, so
    // a relative /api on the server falls through to the SPA catch-all. Hitting
    // the backend by absolute URL fixes SSR auth (hard loads / deep links).
    // Override in prod with NUXT_API_BASE.
    apiBase: process.env.NUXT_API_BASE || "http://127.0.0.1:8081",
    platformCatalogFingerprint:
      process.env.NUXT_PLATFORM_CATALOG_FINGERPRINT || "local-unversioned",
    platformEnvironment: process.env.NUXT_PLATFORM_ENVIRONMENT || "local",
    platformCapabilityRequirementsB64:
      process.env.NUXT_PLATFORM_CAPABILITY_REQUIREMENTS_B64 || "W10=",
    platformCompositionDir: process.env.NUXT_PLATFORM_COMPOSITION_DIR || "",
  },
  // Dev-only proxy to the Go backend so the app is same-origin in `nuxt dev`.
  // In Nuxt 4 this lives under `nitro.devProxy` (the top-level `devProxy`
  // key is not honored by the installed nitro version).
  nitro: {
    // Nitro's devProxy STRIPS the matched prefix before forwarding, so each
    // target must re-include it (/api/v1/x → backend /api/v1/x). 127.0.0.1
    // (not localhost) avoids an IPv6/IPv4 resolution split — the Go backend
    // listens on 127.0.0.1.
    // Proxy only the backend's own namespace (/api/v1, not all of /api): Nuxt
    // serves its icon data at /api/_nuxt_icon/*, and a broad /api rule would
    // forward that to the Go backend (404) — breaking every client-rendered
    // icon (e.g. dropdown items that mount on open). Backend routes are all
    // under /api/v1.
    devProxy: {
      "/api/v1": { target: "http://127.0.0.1:8081/api/v1", changeOrigin: true },
      "/oauth2": { target: "http://127.0.0.1:8081/oauth2", changeOrigin: true },
      "/.well-known": {
        target: "http://127.0.0.1:8081/.well-known",
        changeOrigin: true,
      },
    },
  },
  fonts: {
    providers: {
      google: false,
      googleicons: false,
      bunny: false,
      fontshare: false,
    },
  },
  devtools: { enabled: true },
});
