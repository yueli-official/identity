// Populate the auth user once on app init (SSR) so the chrome (login/user
// widget) renders the correct state on first paint.
export default defineNuxtPlugin(async () => {
  // An auth server route can fall through to Nuxt's error renderer when its
  // upstream call fails. Refreshing again while that error page is rendered
  // recursively calls /auth/session and can exhaust the OIDC rate limit.
  if (useRequestURL().pathname.startsWith('/auth/')) return

  const { user, refresh } = useAuth()
  // SSR may refresh or migrate a sealed session through an internal
  // /auth/session request. Its Set-Cookie header belongs to that internal
  // response, not the outer HTML response. Reconcile once after hydration so
  // the browser receives the rotated or product-scoped cookie. The refresh
  // replay grace keeps an SSR-rotated refresh token valid for this late call.
  if (import.meta.client) {
    await refresh()
    return
  }
  if (!user.value) await refresh()
})
