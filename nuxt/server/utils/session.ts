import type { H3Event } from 'h3'
import type { Session } from './oidc'

interface SessionOptions {
  clearOnRefreshFailure?: boolean
}

function refreshFailureCode(error: unknown): string {
  if (!error || typeof error !== 'object') return ''
  const value = error as {
    data?: { error?: unknown; code?: unknown }
    failure?: { code?: unknown }
  }
  const candidate = value.data?.error || value.data?.code || value.failure?.code
  return typeof candidate === 'string' ? candidate : ''
}

export async function sessionForEvent(event: H3Event, _options: SessionOptions = {}): Promise<Session | null> {
  const cfg = oidcConfig(event)
  const session = unseal<Session>(getCookie(event, cfg.cookies.session), cfg.sealSecret)
  if (!session) return null

  if (Date.now() <= session.exp - 30_000) {
    return session
  }
  if (!session.refresh) {
    deleteCookie(event, cfg.cookies.session, { path: '/' })
    return null
  }

  try {
    const tok = await refreshSingleFlight(cfg, session.refresh)
    const next = sessionFromTokens(tok, session)
    setCookie(event, cfg.cookies.session, seal(next, cfg.sealSecret), {
      httpOnly: true,
      secure: cfg.cookieSecure,
      sameSite: 'lax',
      path: '/',
      maxAge: 60 * 60 * 24 * 7
    })
    return next
  } catch (error) {
    if (refreshFailureCode(error) === 'invalid_grant') {
      deleteCookie(event, cfg.cookies.session, { path: '/' })
      return null
    }
    throw error
  }
}

export async function sessionAuthHeaders(event: H3Event): Promise<Record<string, string>> {
  const session = await sessionForEvent(event)
  return session?.access ? { authorization: `Bearer ${session.access}` } : {}
}
