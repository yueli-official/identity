// Nuxt 4 config for the account-center app.
// @nuxt/fonts ships bundled inside @nuxt/ui v4, so it is not listed as a
// separate module here (@nuxt/ui registers it automatically).
const identityApiBase = (
  process.env.NUXT_API_BASE || "http://127.0.0.1:8081"
).replace(/\/+$/, "");
const assetApiBase = (
  process.env.NUXT_ASSET_BASE || "http://127.0.0.1:8082"
).replace(/\/+$/, "");

export default defineNuxtConfig({
  modules: ["@nuxt/ui", "@yueli/ui"],
  icon: {
    provider: "none",
    fallbackToApi: false,
    serverBundle: { collections: ["tabler"] },
    clientBundle: {
      scan: {
        globInclude: [
          "app/**/*.{vue,js,mjs,ts,jsx,tsx}",
          "node_modules/@yueli/**/*.{vue,js,mjs,ts,jsx,tsx}",
        ],
        globExclude: [
          "test/**",
          "tests/**",
          "coverage/**",
          "dist/**",
          ".nuxt/**",
          ".output/**",
          ".*",
        ],
      },
      sizeLimitKb: 256,
    },
  },
  css: ["~/assets/css/main.css"],
  vite: {
    optimizeDeps: {
      include: ["zod"],
    },
  },
  // The local dev server does not publish Nuxt's generated app-manifest at
  // /_nuxt/builds/meta/dev.json. Disable the client poller so the first OIDC
  // visit is deterministic and does not report a framework-only 404.
  experimental: {
    appManifest: false,
  },
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
    apiBase: identityApiBase,
    assetBase: assetApiBase,
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
      "/api/v1": { target: `${identityApiBase}/api/v1`, changeOrigin: true },
      "/oauth2": { target: `${identityApiBase}/oauth2`, changeOrigin: true },
      "/.well-known": {
        target: `${identityApiBase}/.well-known`,
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
  // Acceptance and public-profile renders must never include Nuxt's timing
  // overlay. Developers can still use browser devtools without shipping
  // framework diagnostics inside the product surface.
  devtools: { enabled: false },
});
