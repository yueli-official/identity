// gokit envelope: code is a namespaced string; success sentinel is "ok"
// (see packages/gokit/response/envelope.go OK()).
interface Envelope<T> { code: string; data: T; message: string; traceId: string }

export function useApi() {
  async function call<T>(url: string, opts?: Parameters<typeof $fetch>[1]): Promise<T> {
    // During SSR, `credentials: 'include'` does NOT forward the inbound
    // id_session cookie (that's a browser-only concept), so server-side calls
    // (e.g. the auth middleware on a hard navigation) would look unauthenticated.
    // Forward the request cookie header explicitly on the server.
    const cookieHeaders = import.meta.server ? useRequestHeaders(['cookie']) : {}
    const res = await $fetch<Envelope<T>>(url, {
      credentials: 'include',
      ...opts,
      headers: { ...cookieHeaders, ...(opts?.headers as Record<string, string> | undefined) },
    })
    if (res.code !== 'ok') {
      throw createError({ statusCode: 400, statusMessage: res.message, data: res })
    }
    return res.data
  }
  return { call }
}
