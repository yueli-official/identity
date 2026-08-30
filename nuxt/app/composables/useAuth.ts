export interface AuthUser {
  sub: string
  userKey?: string
  email?: string
  name?: string
  avatar?: string
  roles?: string[]
}

interface AuthSessionResponse {
  user: AuthUser | null
  reauthenticate?: boolean
}

const CLIENT_REFRESH_RETRY_DELAYS = [1_000, 3_000, 10_000] as const
let clientRefreshRetryAttempt = 0
let clientRefreshRetryTimer: ReturnType<typeof setTimeout> | undefined
let clientRefreshPromise: Promise<AuthUser | null> | undefined

function isBrowser() {
  return typeof window !== 'undefined'
}

function cancelClientRefreshRetry() {
  if (clientRefreshRetryTimer !== undefined) clearTimeout(clientRefreshRetryTimer)
  clientRefreshRetryTimer = undefined
  clientRefreshRetryAttempt = 0
}

function scheduleClientRefreshRetry(refresh: () => Promise<AuthUser | null>) {
  if (!isBrowser() || clientRefreshRetryTimer !== undefined || clientRefreshRetryAttempt >= CLIENT_REFRESH_RETRY_DELAYS.length) return
  const delay = CLIENT_REFRESH_RETRY_DELAYS[clientRefreshRetryAttempt++]!
  clientRefreshRetryTimer = setTimeout(() => {
    clientRefreshRetryTimer = undefined
    void refresh()
  }, delay)
}

// useAuth exposes the BFF session: the user (display claims, never tokens),
// login (full navigation to the BFF /auth/login), and logout.
export function useAuth() {
	const runtime = useRuntimeConfig()
  const user = useState<AuthUser | null>('rs-auth-user', () => null)
  const reauthenticate = useState<boolean>('rs-auth-reauthenticate', () => false)

  async function refresh() {
    async function performRefresh() {
      const headers = import.meta.server ? useRequestHeaders(['cookie']) : undefined
      try {
        const res = await $fetch<AuthSessionResponse>('/auth/session', { headers })
        user.value = res.user
        const shouldReauthenticate = !res.user && (reauthenticate.value || Boolean(res.reauthenticate))
        reauthenticate.value = shouldReauthenticate
        if (isBrowser() && shouldReauthenticate) {
          cancelClientRefreshRetry()
          const returnTo = window.location.pathname + window.location.search
          reauthenticate.value = false
          await navigateTo(`/auth/login?return_to=${encodeURIComponent(returnTo)}`, { external: true })
          return user.value
        }
        if (isBrowser()) cancelClientRefreshRetry()
      } catch {
        // A transient Identity/BFF failure is not a logout. Keep the last
        // confirmed display state and let the shared client retry recover it.
        scheduleClientRefreshRetry(refresh)
      }
      return user.value
    }

    if (!isBrowser()) return await performRefresh()
    if (clientRefreshPromise) return await clientRefreshPromise
    clientRefreshPromise = performRefresh().finally(() => { clientRefreshPromise = undefined })
    return await clientRefreshPromise
  }

  function login(returnTo?: string) {
    cancelClientRefreshRetry()
    reauthenticate.value = false
    const rt = returnTo ?? (import.meta.client ? window.location.pathname + window.location.search : '/')
    return navigateTo(`/auth/login?return_to=${encodeURIComponent(rt)}`, { external: true })
  }

  async function logout() {
    cancelClientRefreshRetry()
    reauthenticate.value = false
    // POST clears the local session and returns the IdP end_session URL; navigate
    // there so the IdP also drops its id_session (full logout, no silent SSO
    // re-login), then bounces back to this site via post_logout_redirect_uri.
    const res = await $fetch<{ ok: boolean; endSession?: string }>('/auth/logout', { method: 'POST' }).catch(() => null)
    user.value = null
    return navigateTo(res?.endSession || '/', { external: true })
  }

  const loggedIn = computed(() => !!user.value)
	const operatorSubs = computed(() =>
		String(runtime.public.operatorSubs || '')
			.split(',')
			.map((sub) => sub.trim())
			.filter(Boolean)
	)
	const isAdmin = computed(() => !!user.value && operatorSubs.value.includes(user.value.userKey || user.value.sub))
  return { user, loggedIn, isAdmin, refresh, login, logout }
}
