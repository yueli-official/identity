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
  roles: string[]
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
  // 站群超级管理员:identity 全局 admin 角色(网关/控制台用)。
  const isAdmin = computed(() => me.value?.roles?.includes('admin') ?? false)
  return { me, refresh, logout, isAdmin }
}
