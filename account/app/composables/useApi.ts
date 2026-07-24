interface Envelope<T> {
  code: string;
  data: T;
  message: string;
  traceId: string;
}

export function useApi() {
  async function call<T>(
    url: string,
    opts?: Parameters<typeof $fetch>[1],
  ): Promise<T> {
    // During SSR, `credentials: 'include'` does NOT forward the inbound
    // id_session cookie (that's a browser-only concept), so server-side calls
    // (e.g. the auth middleware on a hard navigation) would look unauthenticated.
    // Forward the request cookie header explicitly on the server.
    const cookieHeaders = import.meta.server
      ? useRequestHeaders(["cookie"])
      : {};
    // On the server the dev proxy doesn't apply (internal SSR $fetch bypasses
    // it), so call the backend by absolute URL; on the client use a relative URL
    // so the request is same-origin and goes through the proxy.
    const base = import.meta.server ? useRuntimeConfig().apiBase : "";
    const res = await $fetch<Envelope<T> | T>(base + url, {
      credentials: "include",
      ...opts,
      headers: {
        ...cookieHeaders,
        ...(opts?.headers as Record<string, string> | undefined),
      },
    });
    if (res && typeof res === "object" && "code" in res) {
      if (res.code !== "ok") {
        throw createError({
          statusCode: 400,
          statusMessage: res.message,
          data: res,
        });
      }
      return res.data;
    }
    return res as T;
  }
  return { call };
}
