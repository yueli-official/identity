// Receives the IdP redirect, validates state, exchanges the code for tokens
// (server-side, with the PKCE verifier), seals them into the session cookie, and
// returns the user to where they started.
interface Tx {
  verifier: string
  state: string
  nonce: string
  returnTo: string
}

export default defineEventHandler(async (event) => {
  const cfg = oidcConfig(event)
  const q = getQuery(event)

  const tx = unseal<Tx>(getCookie(event, cfg.cookies.transaction), cfg.sealSecret)
  deleteCookie(event, cfg.cookies.transaction, { path: '/' })
  if (!tx || !q.code || q.state !== tx.state) {
    throw createError({ statusCode: 400, statusMessage: 'invalid oauth state' })
  }

  const tok = await exchangeCode(cfg, q.code as string, tx.verifier)
  const claims = await verifyIdentityIdToken(tok.id_token, cfg, tx.nonce)
  const session = sessionFromTokens(tok, undefined, claims)

  setCookie(event, cfg.cookies.session, seal(session, cfg.sealSecret), {
    httpOnly: true,
    secure: cfg.cookieSecure,
    sameSite: 'lax',
    path: '/',
    maxAge: 60 * 60 * 24 * 7
  })

  return sendRedirect(event, safeReturnTo(tx.returnTo))
})
