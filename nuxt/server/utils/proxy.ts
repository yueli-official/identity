import type { H3Event } from "h3";

// Caddy is the only published production ingress and normalizes X-Forwarded-For.
// Rebuild the header at each BFF hop so callers cannot append arbitrary chains.
export function platformProxyHeaders(
  event: H3Event,
  headers: Record<string, string> = {},
): Record<string, string> {
  const clientIP = getRequestIP(event, { xForwardedFor: true });
  return {
    ...headers,
    ...(clientIP ? { "x-forwarded-for": clientIP } : {}),
  };
}
