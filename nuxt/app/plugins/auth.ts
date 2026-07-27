// Populate the auth user once on app init (SSR) so the chrome (login/user
// widget) renders the correct state on first paint.
export default defineNuxtPlugin(async () => {
  const { user, refresh } = useAuth()
  if (!user.value) await refresh()
})
