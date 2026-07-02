// Receives the IdP redirect, validates state, exchanges the code for tokens
// (server-side, with the PKCE verifier), seals them into the session cookie, and
// returns the user to where they started.
interface Tx {
  verifier: string
  state: string
  returnTo: string
}

export default defineEventHandler(async (event) => {
  const cfg = oidcConfig(event)
  const q = getQuery(event)

  const tx = unseal<Tx>(getCookie(event, TX_COOKIE), cfg.sealSecret)
  deleteCookie(event, TX_COOKIE, { path: '/' })
  if (!tx || !q.code || q.state !== tx.state) {
    throw createError({ statusCode: 400, statusMessage: 'invalid oauth state' })
  }

  const tok = await exchangeCode(cfg, q.code as string, tx.verifier)
  const session = sessionFromTokens(tok)

  setCookie(event, SESSION_COOKIE, seal(session, cfg.sealSecret), {
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
    maxAge: 60 * 60 * 24 * 7
  })

  return sendRedirect(event, safeReturnTo(tx.returnTo))
})
