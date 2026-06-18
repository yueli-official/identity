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
