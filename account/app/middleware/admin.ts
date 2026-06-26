// Gate the admin console: must be authenticated AND hold the global `admin`
// role. Non-admins are bounced to the account center rather than the login page
// (they're logged in, just not privileged).
export default defineNuxtRouteMiddleware(async (to) => {
  const { me, refresh, isAdmin } = useSession()
  if (!me.value) await refresh()
  if (!me.value) {
    return navigateTo(`/login?return_to=${encodeURIComponent(to.fullPath)}`)
  }
  if (!isAdmin.value) {
    return navigateTo('/')
  }
})
