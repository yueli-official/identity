export default defineEventHandler((event) => {
  const base = useRuntimeConfig().apiBase.replace(/\/$/, "");
  return proxyRequest(event, base + event.path, {
    headers: platformProxyHeaders(event),
    streamRequest: true,
  });
});
