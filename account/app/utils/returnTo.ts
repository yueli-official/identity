// Only allow same-origin relative paths as return_to (defense vs open redirect).
// Reject "//" and "/\" (scheme-relative once a browser normalizes the backslash)
// and any backslash / control char.
export function safeReturnTo(raw: string | null | undefined): string {
  if (!raw) return '/'
  if (raw[0] !== '/') return '/'
  if (/[\\\r\n\t]/.test(raw)) return '/'
  if (raw[1] === '/') return '/'
  return raw
}

// A valid central session may continue an ordinary OIDC authorization without
// asking for credentials again. Explicit re-authentication requests and an
// in-progress MFA transaction must still remain on the login page.
export function existingSessionReturnTo(
  raw: string | null | undefined,
  hasMFATransaction = false,
): string | null {
  if (hasMFATransaction) return null
  const target = safeReturnTo(raw)
  const parsed = new URL(target, 'http://account.invalid')
  const prompts = (parsed.searchParams.get('prompt') ?? '').split(/\s+/)
  if (prompts.includes('login') || parsed.searchParams.get('max_age') === '0') return null
  return target
}
