export default defineNuxtRouteMiddleware(async (to) => {
  const { me, refresh } = useSession()
  if (!me.value) await refresh()
  if (!me.value) {
    return navigateTo(`/login?return_to=${encodeURIComponent(to.fullPath)}`)
  }
})
