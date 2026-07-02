// Begins Authorization Code + PKCE: stash the verifier/state/return_to in a
// short-lived sealed cookie, then redirect to the IdP authorize endpoint.
export default defineEventHandler((event) => {
  const cfg = oidcConfig(event)
  const { verifier, challenge } = pkce()
  const state = randomToken()
  const returnTo = safeReturnTo(getQuery(event).return_to as string)

  setCookie(event, TX_COOKIE, seal({ verifier, state, returnTo }, cfg.sealSecret), {
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
    maxAge: 600
  })

  const url = `${cfg.authorizeEndpoint}?${new URLSearchParams({
    response_type: 'code',
    client_id: cfg.clientId,
    redirect_uri: cfg.redirectUri,
    scope: cfg.scopes,
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256'
  }).toString()}`

  return sendRedirect(event, url)
})
