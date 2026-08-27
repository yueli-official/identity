import type { H3Event } from "h3";
import type { Session } from "./oidc";
import { decodeProductSession, encodeProductSession } from "./product-session";

interface SessionOptions {
  clearOnRefreshFailure?: boolean;
}

const LEGACY_SESSION_COOKIE_NAMES = ["rs_session"] as const;

function refreshFailureCode(error: unknown): string {
  if (!error || typeof error !== "object") return "";
  const value = error as {
    data?: { error?: unknown; code?: unknown };
    failure?: { code?: unknown };
  };
  const candidate =
    value.data?.error || value.data?.code || value.failure?.code;
  return typeof candidate === "string" ? candidate : "";
}

export async function sessionForEvent(
  event: H3Event,
  _options: SessionOptions = {},
): Promise<Session | null> {
  const cfg = oidcConfig(event);
  let sourceCookie = cfg.cookies.session;
  let decoded = decodeProductSession(
    getCookie(event, cfg.cookies.session),
    cfg,
  );
  if (!decoded) {
    for (const legacyCookie of LEGACY_SESSION_COOKIE_NAMES) {
      decoded = decodeProductSession(getCookie(event, legacyCookie), cfg);
      if (decoded) {
        sourceCookie = legacyCookie;
        break;
      }
    }
  }
  if (!decoded) return null;
  const session = decoded.session;

  function writeSession(value: Session) {
    setCookie(event, cfg.cookies.session, encodeProductSession(value, cfg), {
      httpOnly: true,
      secure: cfg.cookieSecure,
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 7,
    });
    if (sourceCookie !== cfg.cookies.session) {
      deleteCookie(event, sourceCookie, { path: "/" });
    }
  }

  function clearSourceCookie() {
    deleteCookie(event, sourceCookie, { path: "/" });
  }

  if (Date.now() <= session.exp - 30_000) {
    if (decoded.needsMigration || sourceCookie !== cfg.cookies.session) {
      writeSession(session);
    }
    return session;
  }
  if (!session.refresh) {
    clearSourceCookie();
    return null;
  }

  try {
    const tok = await refreshSingleFlight(cfg, session.refresh);
    const next = sessionFromTokens(tok, session);
    writeSession(next);
    return next;
  } catch (error) {
    if (refreshFailureCode(error) === "invalid_grant") {
      clearSourceCookie();
      return null;
    }
    throw error;
  }
}

export async function sessionAuthHeaders(
  event: H3Event,
): Promise<Record<string, string>> {
  const session = await sessionForEvent(event);
  return session?.access ? { authorization: `Bearer ${session.access}` } : {};
}
