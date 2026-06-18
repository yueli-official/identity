// Only allow same-origin relative paths as return_to (defense vs open redirect).
export function safeReturnTo(raw: string | null | undefined): string {
  if (!raw) return '/'
  if (raw.startsWith('/') && !raw.startsWith('//')) return raw
  return '/'
}
