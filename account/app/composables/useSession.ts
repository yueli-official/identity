import type { MediaRef } from '~/utils/media'

export interface SocialLink {
  label: string
  url: string
}

interface SessionUser {
  userKey: string
  email: string
  emailVerified: boolean
  displayName: string
  handle: string
  avatar?: MediaRef
  cover?: MediaRef
  bio: string
  socialLinks: SocialLink[]
  roles: string[]
}

export interface Me extends SessionUser {
  avatarUrl: string
  coverUrl: string
}

export function useSession() {
  const me = useState<Me | null>('me', () => null)
  const { call } = useApi()
  async function refresh() {
    try {
      const user = await call<SessionUser>('/api/v1/session/me')
      me.value = {
        ...user,
        avatarUrl: userMediaUrl(user.avatar, 'thumbnail'),
        coverUrl: userMediaUrl(user.cover, 'cover')
      }
    }
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
