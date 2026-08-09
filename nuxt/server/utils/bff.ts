import type { BffCredential, BffTarget } from "@yueli/nuxt-runtime/server";

export function identityBffTarget(base: string): BffTarget {
  const url = new URL(base);
  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new TypeError("Invalid private BFF target");
  }
  const basePath =
    url.pathname === "/" ? "" : url.pathname.replace(/\/$/, "");
  return { origin: url.origin, pathPrefix: `${basePath}/api/v1` };
}

export function identityBffCredential(
  headers: Readonly<Record<string, string>>,
): BffCredential {
  const authorization = headers.authorization;
  return authorization?.startsWith("Bearer ")
    ? { kind: "bearer", token: authorization.slice("Bearer ".length) }
    : { kind: "anonymous" };
}
