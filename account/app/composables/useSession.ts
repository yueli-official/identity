export interface SocialLink {
  label: string
  url: string
}

export interface Me {
  id: string
  email: string
  emailVerified: boolean
  displayName: string
  username: string
  avatarUrl: string
  coverUrl: string
  bio: string
  socialLinks: SocialLink[]
}

export function useSession() {
  const me = useState<Me | null>('me', () => null)
  const { call } = useApi()
  async function refresh() {
    try { me.value = await call<Me>('/api/v1/session/me') }
    catch { me.value = null }
    return me.value
  }
  async function logout() {
    await call('/api/v1/auth/logout', { method: 'POST' })
    me.value = null
  }
  return { me, refresh, logout }
}
