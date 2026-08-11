export default defineEventHandler((event) => {
  if (!event.path.startsWith("/.well-known/")) return;

  const base = useRuntimeConfig(event).apiBase.replace(/\/$/, "");
  return proxyRequest(event, base + event.path, {
    headers: platformProxyHeaders(event),
    streamRequest: true,
  });
});
