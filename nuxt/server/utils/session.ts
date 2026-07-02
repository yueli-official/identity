import type { H3Event } from 'h3'
import type { Session } from './oidc'

interface SessionOptions {
  clearOnRefreshFailure?: boolean
}

export async function sessionForEvent(event: H3Event, options: SessionOptions = {}): Promise<Session | null> {
  const cfg = oidcConfig(event)
  const session = unseal<Session>(getCookie(event, SESSION_COOKIE), cfg.sealSecret)
  if (!session) return null

  if (!session.refresh || Date.now() <= session.exp - 30_000) {
    return session
  }

  try {
    const tok = await refreshSingleFlight(cfg, session.refresh)
    const next = sessionFromTokens(tok, session)
    setCookie(event, SESSION_COOKIE, seal(next, cfg.sealSecret), {
      httpOnly: true,
      sameSite: 'lax',
      path: '/',
      maxAge: 60 * 60 * 24 * 7
    })
    return next
  } catch {
    if (options.clearOnRefreshFailure) {
      deleteCookie(event, SESSION_COOKIE, { path: '/' })
      return null
    }
    return session
  }
}

export async function sessionAuthHeaders(event: H3Event): Promise<Record<string, string>> {
  const session = await sessionForEvent(event)
  return session?.access ? { authorization: `Bearer ${session.access}` } : {}
}
