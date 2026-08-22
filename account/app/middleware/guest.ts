import { existingSessionReturnTo } from '~/utils/returnTo'

export default defineNuxtRouteMiddleware(async (to) => {
  // Initial OIDC entry is a full-page navigation. Probe on the server so an
  // anonymous 401 never becomes a browser console error, and do not repeat the
  // same probe during hydration.
  if (import.meta.client) return
  const target = existingSessionReturnTo(
    to.query.return_to as string,
    Boolean(to.query.mfa_transaction),
  )
  if (!target) return
  const { refresh } = useSession()
  if (await refresh()) {
    return navigateTo(target, { external: true })
  }
})
