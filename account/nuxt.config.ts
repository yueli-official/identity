// Nuxt 4 config for the account-center app.
// @nuxt/fonts ships bundled inside @nuxt/ui v4, so it is not listed as a
// separate module here (@nuxt/ui registers it automatically).
export default defineNuxtConfig({
  modules: ['@nuxt/ui'],
  css: ['~/assets/css/main.css'],
  // Dev-only proxy to the Go backend so the app is same-origin in `nuxt dev`.
  // In Nuxt 4 this lives under `nitro.devProxy` (the top-level `devProxy`
  // key is not honored by the installed nitro version).
  nitro: {
    // Nitro's devProxy STRIPS the matched prefix before forwarding, so each
    // target must re-include it (/api/v1/x → backend /api/v1/x). 127.0.0.1
    // (not localhost) avoids an IPv6/IPv4 resolution split — the Go backend
    // listens on 127.0.0.1.
    devProxy: {
      '/api': { target: 'http://127.0.0.1:8081/api', changeOrigin: true },
      '/oauth2': { target: 'http://127.0.0.1:8081/oauth2', changeOrigin: true },
      '/.well-known': { target: 'http://127.0.0.1:8081/.well-known', changeOrigin: true }
    }
  },
  devtools: { enabled: true }
})
