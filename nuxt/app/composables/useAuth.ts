export interface AuthUser {
  sub: string
  userKey?: string
  email?: string
  name?: string
  avatar?: string
  roles?: string[]
}

// useAuth exposes the BFF session: the user (display claims, never tokens),
// login (full navigation to the BFF /auth/login), and logout.
export function useAuth() {
	const runtime = useRuntimeConfig()
  const user = useState<AuthUser | null>('rs-auth-user', () => null)

  async function refresh() {
    const headers = import.meta.server ? useRequestHeaders(['cookie']) : undefined
    try {
      const res = await $fetch<{ user: AuthUser | null }>('/auth/session', { headers })
      user.value = res.user
    } catch {
      // A transient Identity/BFF failure is not a logout. Keep the last
      // confirmed display state; a later refresh can renew the session.
    }
    return user.value
  }

  function login(returnTo?: string) {
    const rt = returnTo ?? (import.meta.client ? window.location.pathname + window.location.search : '/')
    return navigateTo(`/auth/login?return_to=${encodeURIComponent(rt)}`, { external: true })
  }

  async function logout() {
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
