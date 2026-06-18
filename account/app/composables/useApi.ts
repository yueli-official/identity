// gokit envelope: code is a namespaced string; success sentinel is "ok"
// (see packages/gokit/response/envelope.go OK()).
interface Envelope<T> { code: string; data: T; message: string; traceId: string }

export function useApi() {
  async function call<T>(url: string, opts?: Parameters<typeof $fetch>[1]): Promise<T> {
    const res = await $fetch<Envelope<T>>(url, { credentials: 'include', ...opts })
    if (res.code !== 'ok') {
      throw createError({ statusCode: 400, statusMessage: res.message, data: res })
    }
    return res.data
  }
  return { call }
}
