import type { H3Event } from "h3";

export function platformProxyHeaders(event: H3Event): Record<string, string> {
  const clientIP = getRequestIP(event, { xForwardedFor: true });
  return clientIP ? { "x-forwarded-for": clientIP } : {};
}
