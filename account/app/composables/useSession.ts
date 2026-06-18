export function useSession() {
  const me = useState<{ id: string; email: string; emailVerified: boolean } | null>('me', () => null)
  const { call } = useApi()
  async function refresh() {
    try { me.value = await call('/api/v1/session/me') }
    catch { me.value = null }
    return me.value
  }
  async function logout() {
    await call('/api/v1/auth/logout', { method: 'POST' })
    me.value = null
  }
  return { me, refresh, logout }
}
