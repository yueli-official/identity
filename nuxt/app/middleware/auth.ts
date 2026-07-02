// Route middleware for protected pages: definePageMeta({ middleware: 'auth' }).
// Redirects to the BFF login (which bounces to the IdP) when not signed in.
export default defineNuxtRouteMiddleware(async (to) => {
  const { user, refresh } = useAuth()
  if (!user.value) await refresh()
  if (!user.value) {
    return navigateTo(`/auth/login?return_to=${encodeURIComponent(to.fullPath)}`, { external: true })
  }
})
