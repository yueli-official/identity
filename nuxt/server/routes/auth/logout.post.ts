// RP-initiated logout: clear the local sealed session, then hand the client the
// IdP end_session URL so the browser can also drop the IdP `id_session` — without
// that, the next /authorize would silently re-issue tokens (SSO) and "log out"
// would only last until the next click. post_logout_redirect_uri brings the user
// back to this site, logged out. (Local-only logout is the fallback when the IdP
// has no end_session.)
export default defineEventHandler((event) => {
  const cfg = oidcConfig(event)
  deleteCookie(event, SESSION_COOKIE, { path: '/' })

  const origin = new URL(cfg.redirectUri).origin
  const endSession = `${cfg.endSessionEndpoint}?post_logout_redirect_uri=${encodeURIComponent(origin + '/')}`
  return { ok: true, endSession }
})
