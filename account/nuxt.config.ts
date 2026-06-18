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
    devProxy: {
      '/api': { target: 'http://localhost:8081', changeOrigin: true },
      '/oauth2': { target: 'http://localhost:8081', changeOrigin: true },
      '/.well-known': { target: 'http://localhost:8081', changeOrigin: true }
    }
  },
  runtimeConfig: { public: { apiBase: '' } },
  devtools: { enabled: true }
})
