export interface CanonicalOriginRequest {
  requestURL: URL;
  redirectURI: string;
  method: string;
  accept: string;
}

const INTERNAL_PREFIXES = [
  "/api/",
  "/asset-api/",
  "/auth/",
  "/identity-api/",
  "/media/",
  "/_nuxt/",
];

export function canonicalPageURL(input: CanonicalOriginRequest): string | null {
  const method = input.method.toUpperCase();
  if (method !== "GET" && method !== "HEAD") return null;
  if (!input.accept.toLowerCase().includes("text/html")) return null;
  if (
    input.requestURL.pathname === "/healthz" ||
    INTERNAL_PREFIXES.some((prefix) => input.requestURL.pathname.startsWith(prefix))
  ) {
    return null;
  }

  let canonicalOrigin: string;
  try {
    canonicalOrigin = new URL(input.redirectURI).origin;
  } catch {
    return null;
  }
  if (canonicalOrigin === "null" || input.requestURL.origin === canonicalOrigin) {
    return null;
  }
  return `${canonicalOrigin}${input.requestURL.pathname}${input.requestURL.search}`;
}
