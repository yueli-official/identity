import type { H3Event } from "h3";

interface Envelope<T> {
  code: string;
  message: string;
  data?: T;
}

interface GuestSessionCreated {
  subjectId: string;
  sessionToken: string;
  effectiveTtlSeconds: number;
  expiresAt: string;
}

interface GuestTokenIssued {
  accessToken: string;
  expiresInSeconds: number;
}

const DEV_GUEST_COOKIE = "yueli_guest";
const SECURE_GUEST_COOKIE = "__Host-yueli_guest";

function guestConfig(event: H3Event) {
  const runtime = useRuntimeConfig(event);
  const issuer = String(runtime.public.oidcIssuer || "").replace(/\/$/, "");
  return {
    clientId: String(runtime.public.oidcClientId || ""),
    issuer,
    requestedTtlSeconds: Number(runtime.guestSessionTtlSeconds || 0),
    secureCookie: Boolean(runtime.guestCookieSecure),
  };
}

async function identityRequest<T>(
  url: string,
  body: Record<string, unknown>,
): Promise<T> {
  const response = await $fetch<Envelope<T>>(url, { method: "POST", body });
  if (response.code !== "ok" || !response.data) {
    throw createError({
      statusCode: 502,
      statusMessage:
        response.message || "Identity guest session request failed",
    });
  }
  return response.data;
}

function cookieName(secure: boolean) {
  return secure ? SECURE_GUEST_COOKIE : DEV_GUEST_COOKIE;
}

async function guestSessionToken(event: H3Event): Promise<string | null> {
  const cfg = guestConfig(event);
  if (
    !cfg.clientId ||
    !cfg.issuer ||
    !Number.isSafeInteger(cfg.requestedTtlSeconds) ||
    cfg.requestedTtlSeconds <= 0
  ) {
    return null;
  }
  const name = cookieName(cfg.secureCookie);
  const existing = getCookie(event, name);
  if (existing) return existing;

  const created = await identityRequest<GuestSessionCreated>(
    `${cfg.issuer}/api/v1/guest/sessions`,
    {
      clientId: cfg.clientId,
      requestedTtlSeconds: cfg.requestedTtlSeconds,
    },
  );
  if (!created.sessionToken || created.effectiveTtlSeconds <= 0) {
    throw createError({
      statusCode: 502,
      statusMessage: "Identity returned an invalid guest session",
    });
  }
  setCookie(event, name, created.sessionToken, {
    httpOnly: true,
    secure: cfg.secureCookie,
    sameSite: "lax",
    path: "/",
    maxAge: created.effectiveTtlSeconds,
  });
  return created.sessionToken;
}

export async function guestSessionAuthHeaders(
  event: H3Event,
  audience: string,
): Promise<Record<string, string>> {
  const cfg = guestConfig(event);
  const sessionToken = await guestSessionToken(event);
  if (!sessionToken) return {};
  const issued = await identityRequest<GuestTokenIssued>(
    `${cfg.issuer}/api/v1/guest/tokens`,
    {
      clientId: cfg.clientId,
      sessionToken,
      audience,
    },
  );
  return issued.accessToken
    ? { authorization: `Bearer ${issued.accessToken}` }
    : {};
}

export async function subjectAuthHeaders(
  event: H3Event,
  audience: string,
): Promise<Record<string, string>> {
  const user = await sessionAuthHeaders(event);
  return user.authorization ? user : guestSessionAuthHeaders(event, audience);
}
