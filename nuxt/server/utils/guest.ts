import type { H3Event } from "h3";

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

interface GuestClaimIssued {
  subjectId: string;
  userId: string;
  claimToken: string;
}

interface GuestClaimTarget {
  audience: string;
  base: string;
  path: string;
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
  headers: Record<string, string> = {},
): Promise<T> {
  return await $fetch<T>(url, {
    method: "POST",
    body,
    headers,
  });
}

async function claimResource(target: GuestClaimTarget, claimToken: string) {
  await $fetch<{ claimed: number }>(
    `${target.base.replace(/\/$/, "")}${target.path}`,
    { method: "POST", headers: { authorization: `Bearer ${claimToken}` } },
  );
}

function cookieName(secure: boolean) {
  return secure ? SECURE_GUEST_COOKIE : DEV_GUEST_COOKIE;
}

function isInvalidGuestSession(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const failure = error as {
    data?: { code?: unknown };
    response?: { _data?: { code?: unknown } };
    failure?: { code?: unknown };
  };
  return (
    failure.data?.code === "identity.guest_session_invalid" ||
    failure.response?._data?.code === "identity.guest_session_invalid" ||
    failure.failure?.code === "identity.guest_session_invalid"
  );
}

async function guestSessionToken(
  event: H3Event,
  createIfMissing: boolean,
  ignoreExisting = false,
): Promise<string | null> {
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
  const existing = ignoreExisting ? undefined : getCookie(event, name);
  if (existing) return existing;
  if (!createIfMissing) return null;

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
  createIfMissing = true,
): Promise<Record<string, string>> {
  const cfg = guestConfig(event);
  const name = cookieName(cfg.secureCookie);
  const hadExistingSession = Boolean(getCookie(event, name));
  const sessionToken = await guestSessionToken(event, createIfMissing);
  if (!sessionToken) return {};
  const issue = (token: string) =>
    identityRequest<GuestTokenIssued>(`${cfg.issuer}/api/v1/guest/tokens`, {
      clientId: cfg.clientId,
      sessionToken: token,
      audience,
    });
  let issued: GuestTokenIssued;
  try {
    issued = await issue(sessionToken);
  } catch (error) {
    if (!hadExistingSession || !isInvalidGuestSession(error)) throw error;
    deleteCookie(event, name, { path: "/" });
    // Response cookie mutations do not change the immutable Cookie header on
    // this request, so renewal must explicitly bypass the stale request value.
    const replacement = await guestSessionToken(event, true, true);
    if (!replacement) return {};
    issued = await issue(replacement);
  }
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

export async function claimGuestSessionForEvent(
  event: H3Event,
  userAccessToken: string,
): Promise<boolean> {
  const runtime = useRuntimeConfig(event);
  const cfg = guestConfig(event);
  const targets = (runtime.guestClaimTargets || []) as GuestClaimTarget[];
  if (!userAccessToken || !targets.length) return false;

  const name = cookieName(cfg.secureCookie);
  const sessionToken = getCookie(event, name);
  if (!sessionToken) return false;

  for (const target of targets) {
    if (!target.audience || !target.base || !target.path) continue;
    const claim = await identityRequest<GuestClaimIssued>(
      `${cfg.issuer}/api/v1/guest/sessions/claim`,
      {
        clientId: cfg.clientId,
        sessionToken,
        audience: target.audience,
      },
      { authorization: `Bearer ${userAccessToken}` },
    );
    if (!claim.claimToken) {
      throw createError({
        statusCode: 502,
        statusMessage: "Identity returned an invalid guest claim",
      });
    }
    await claimResource(target, claim.claimToken);
  }

  deleteCookie(event, name, { path: "/" });
  return true;
}
