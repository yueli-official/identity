// BFF proxy: forwards /api/v1/** to the downstream service, injecting the
// session's access token as a Bearer header (refreshing it first if near expiry).
// Anonymous requests (no session) are forwarded without a token, so public
// endpoints keep working logged-out.
export default defineEventHandler(async (event) => {
  const cfg = oidcConfig(event);
  const method = getMethod(event).toUpperCase();
  const authHeaders = ["GET", "HEAD", "OPTIONS"].includes(method)
    ? await sessionAuthHeaders(event)
    : await subjectAuthHeaders(event, cfg.clientId);
  const headers = platformProxyHeaders(event, authHeaders);
  return await proxyRequest(event, cfg.downstreamBase + event.path, {
    headers,
    streamRequest: true,
  });
});
