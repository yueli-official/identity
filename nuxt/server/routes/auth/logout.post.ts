// RP-initiated logout: clear the local sealed session, then hand the client the
// IdP end_session URL so the browser can also drop the IdP `id_session` — without
// that, the next /authorize would silently re-issue tokens (SSO) and "log out"
// would only last until the next click. post_logout_redirect_uri brings the user
// back to this site, logged out. (Local-only logout is the fallback when the IdP
// has no end_session.)
export default defineEventHandler((event) => {
  const cfg = oidcConfig(event)
  const origin = getRequestHeader(event, "origin");
  if (origin && origin !== getRequestURL(event).origin) {
    throw createError({
      statusCode: 403,
      statusMessage: "Cross-origin logout is not allowed",
    });
  }
  deleteCookie(event, cfg.cookies.session, { path: '/' })

  const query = new URLSearchParams({
    client_id: cfg.clientId,
    post_logout_redirect_uri: cfg.postLogoutRedirectUri
  })
  const endSession = `${cfg.endSessionEndpoint}?${query}`
  return { ok: true, endSession }
})
