/// <reference types="node" />
// OIDC BFF helpers for consumer sites. No external
// OIDC/jose lib — Node's crypto does PKCE (sha256) + a sealed AES-256-GCM cookie.
// (The node reference makes node:crypto + Buffer resolve under each app's
// vue-tsc, which checks this layer's server code with the client tsconfig.)
// Tokens live only server-side; the browser only ever holds the sealed cookie.
import {
  createCipheriv,
  createDecipheriv,
  createHash,
  createPublicKey,
  randomBytes,
  verify as verifySignature,
} from "node:crypto";
import type { H3Event } from "h3";

export interface OidcCfg {
  issuer: string;
  clientId: string;
  clientSecret: string;
  redirectUri: string;
  postLogoutRedirectUri: string;
  scopes: string;
  authorizeEndpoint: string;
  tokenEndpoint: string;
  jwksEndpoint: string;
  endSessionEndpoint: string;
  downstreamBase: string;
  sealSecret: string;
  sealSecretPrevious?: string;
  cookieSecure: boolean;
  cookies: ProductCookieNames;
}

export interface ProductCookieNames {
  session: string;
  transaction: string;
}

export interface Session {
  access: string;
  refresh?: string;
  exp: number; // epoch ms
  user: { sub: string; userKey?: string; email?: string; name?: string };
}

// OAuth client identifiers are stable, case-sensitive registration identities.
// The safe client label keeps browser diagnostics readable. A 48-bit SHA-256
// prefix still binds the name to the exact, case-sensitive client ID, so two IDs
// that normalize to the same label remain isolated.
export function productCookieNames(clientId: string): ProductCookieNames {
  if (!clientId.trim()) {
    throw new Error(
      "OIDC client ID is required for the product session namespace",
    );
  }
  const label =
    clientId
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .replace(/-web$/g, "")
      .slice(0, 40)
      .replace(/-+$/g, "") || "client";
  const namespace = createHash("sha256")
    .update(clientId)
    .digest("hex")
    .slice(0, 12);
  return {
    session: `ys_${label}_${namespace}`,
    transaction: `yt_${label}_${namespace}`,
  };
}

export function oidcConfig(event: H3Event): OidcCfg {
  const rc = useRuntimeConfig(event);
  const issuer = (rc.public.oidcIssuer as string).replace(/\/$/, "");
  const clientId = rc.public.oidcClientId as string;
  return {
    issuer,
    clientId,
    clientSecret: rc.oidcClientSecret as string,
    redirectUri: rc.public.oidcRedirectUri as string,
    postLogoutRedirectUri: rc.public.oidcPostLogoutRedirectUri as string,
    scopes:
      (rc.public.oidcScopes as string) ||
      "openid profile email roles offline_access",
    authorizeEndpoint: issuer + "/oauth2/authorize",
    tokenEndpoint: issuer + "/oauth2/token",
    jwksEndpoint: issuer + "/oauth2/jwks.json",
    endSessionEndpoint: issuer + "/oauth2/end_session",
    downstreamBase: (rc.downstreamBase as string).replace(/\/$/, ""),
    sealSecret: rc.sealSecret as string,
    sealSecretPrevious: (rc.sealSecretPrevious as string) || "",
    cookieSecure: Boolean(rc.authCookieSecure),
    cookies: productCookieNames(clientId),
  };
}

function b64url(b: Buffer) {
  return b.toString("base64url");
}

// PKCE (S256): a random verifier + its sha256 challenge.
export function pkce() {
  const verifier = b64url(randomBytes(32));
  const challenge = b64url(createHash("sha256").update(verifier).digest());
  return { verifier, challenge };
}

export function randomToken(n = 16) {
  return b64url(randomBytes(n));
}

function keyOf(secret: string) {
  return createHash("sha256").update(secret).digest(); // 32 bytes for AES-256
}

// seal encrypts+authenticates a JSON value into a compact base64url token.
export function seal(data: unknown, secret: string): string {
  const iv = randomBytes(12);
  const c = createCipheriv("aes-256-gcm", keyOf(secret), iv);
  const enc = Buffer.concat([
    c.update(JSON.stringify(data), "utf8"),
    c.final(),
  ]);
  return b64url(Buffer.concat([iv, c.getAuthTag(), enc]));
}

export function unseal<T>(token: string | undefined, secret: string): T | null {
  if (!token) return null;
  try {
    const buf = Buffer.from(token, "base64url");
    const iv = buf.subarray(0, 12);
    const tag = buf.subarray(12, 28);
    const enc = buf.subarray(28);
    const d = createDecipheriv("aes-256-gcm", keyOf(secret), iv);
    d.setAuthTag(tag);
    const dec = Buffer.concat([d.update(enc), d.final()]).toString("utf8");
    return JSON.parse(dec) as T;
  } catch {
    return null;
  }
}

interface JwtHeader {
  alg?: string;
  kid?: string;
}

export interface VerifiedIdentityClaims {
  iss: string;
  sub: string;
  user_key?: string;
  aud: string | string[];
  azp?: string;
  exp: number;
  iat?: number;
  nonce: string;
  email?: string;
  name?: string;
  preferred_username?: string;
  picture?: string;
  roles?: string[];
}

interface JwksDocument {
  keys?: Array<Record<string, unknown>>;
}

const jwksCache = new Map<
  string,
  { expiresAt: number; keys: Array<Record<string, unknown>> }
>();
const JWKS_TTL_MS = 5 * 60 * 1000;

function decodeJwtPart<T>(part: string): T {
  return JSON.parse(Buffer.from(part, "base64url").toString("utf8")) as T;
}

// Product sessions keep the access token inside an authenticated seal and do
// not duplicate mutable role claims in the cookie's display-user projection.
// Re-project only a small, well-formed role list when serving /auth/session;
// product APIs remain the authority for access control.
export function accessTokenRoles(token: string): string[] {
  const parts = token.split(".");
  if (parts.length !== 3 || !parts[1]) return [];

  try {
    const claims = decodeJwtPart<{ roles?: unknown }>(parts[1]);
    if (!Array.isArray(claims.roles)) return [];

    const roles = new Set<string>();
    for (const value of claims.roles) {
      if (typeof value !== "string") continue;
      const role = value.trim();
      if (!role || role.length > 128) continue;
      roles.add(role);
      if (roles.size >= 64) break;
    }
    return [...roles];
  } catch {
    return [];
  }
}

async function fetchJwks(
  cfg: OidcCfg,
  forceRefresh = false,
): Promise<Array<Record<string, unknown>>> {
  const cached = jwksCache.get(cfg.jwksEndpoint);
  if (!forceRefresh && cached && cached.expiresAt > Date.now()) {
    return cached.keys;
  }
  const document = await $fetch<JwksDocument>(cfg.jwksEndpoint, {
    timeout: 2_000,
  });
  const keys = Array.isArray(document.keys) ? document.keys : [];
  if (!keys.length) throw new Error("Identity JWKS is empty");
  jwksCache.set(cfg.jwksEndpoint, {
    expiresAt: Date.now() + JWKS_TTL_MS,
    keys,
  });
  return keys;
}

async function verifyJwtSignature(
  signingInput: string,
  signature: Buffer,
  header: JwtHeader,
  cfg: OidcCfg,
): Promise<boolean> {
  for (const forceRefresh of [false, true]) {
    const keys = await fetchJwks(cfg, forceRefresh);
    const candidates = keys.filter(
      (key) =>
        (!header.kid || key.kid === header.kid) &&
        (!key.alg || key.alg === "RS256") &&
        (!key.use || key.use === "sig") &&
        key.kty === "RSA",
    );
    for (const jwk of candidates) {
      try {
        const key = createPublicKey({ key: jwk, format: "jwk" });
        if (
          verifySignature(
            "RSA-SHA256",
            Buffer.from(signingInput),
            key,
            signature,
          )
        ) {
          return true;
        }
      } catch {
        // Ignore malformed or incompatible keys and try the next candidate.
      }
    }
  }
  return false;
}

export async function verifyIdentityIdToken(
  token: string | undefined,
  cfg: OidcCfg,
  expectedNonce: string,
): Promise<VerifiedIdentityClaims> {
  if (!token) throw new Error("Identity did not return an ID Token");
  const parts = token.split(".");
  if (parts.length !== 3)
    throw new Error("Identity returned an invalid ID Token");
  const encodedHeader = parts[0]!;
  const encodedClaims = parts[1]!;
  const encodedSignature = parts[2]!;
  const header = decodeJwtPart<JwtHeader>(encodedHeader);
  if (header.alg !== "RS256") {
    throw new Error("Identity ID Token uses an unsupported algorithm");
  }
  const verified = await verifyJwtSignature(
    `${encodedHeader}.${encodedClaims}`,
    Buffer.from(encodedSignature, "base64url"),
    header,
    cfg,
  );
  if (!verified) throw new Error("Identity ID Token signature is invalid");

  const claims = decodeJwtPart<VerifiedIdentityClaims>(encodedClaims);
  const now = Math.floor(Date.now() / 1000);
  const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
  if (
    claims.iss !== cfg.issuer ||
    !claims.sub ||
    !audiences.includes(cfg.clientId) ||
    (audiences.length > 1 && claims.azp !== cfg.clientId) ||
    !Number.isFinite(claims.exp) ||
    claims.exp <= now ||
    claims.nonce !== expectedNonce
  ) {
    throw new Error("Identity ID Token claims are invalid");
  }
  return claims;
}

interface TokenResponse {
  access_token: string;
  refresh_token?: string;
  id_token?: string;
  expires_in?: number;
}

export async function exchangeCode(
  cfg: OidcCfg,
  code: string,
  verifier: string,
): Promise<TokenResponse> {
  return await $fetch<TokenResponse>(cfg.tokenEndpoint, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: cfg.redirectUri,
      client_id: cfg.clientId,
      code_verifier: verifier,
      ...(cfg.clientSecret ? { client_secret: cfg.clientSecret } : {}),
    }).toString(),
  });
}

export async function refreshTokens(
  cfg: OidcCfg,
  refresh: string,
): Promise<TokenResponse> {
  return await $fetch<TokenResponse>(cfg.tokenEndpoint, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      refresh_token: refresh,
      client_id: cfg.clientId,
      ...(cfg.clientSecret ? { client_secret: cfg.clientSecret } : {}),
    }).toString(),
  });
}

// Requests that started with the same sealed cookie can reach Nitro a little
// after the first refresh has already completed. Refresh-token rotation makes
// that old token permanently invalid, so removing the single-flight entry as
// soon as the promise settles lets a late request erase the newly issued
// browser cookie. Keep a successful result for a short replay grace window;
// failures remain immediately retryable. The cache is still per Nitro instance.
const REFRESH_REPLAY_GRACE_MS = 5_000;
interface RefreshFlight {
  promise: Promise<TokenResponse>;
  expiresAt: number;
}
const refreshFlights = new Map<string, RefreshFlight>();

export function refreshSingleFlight(
  cfg: OidcCfg,
  refresh: string,
): Promise<TokenResponse> {
  const now = Date.now();
  const existing = refreshFlights.get(refresh);
  if (existing && existing.expiresAt > now) return existing.promise;
  if (existing) refreshFlights.delete(refresh);

  let flight!: RefreshFlight;
  const promise = refreshTokens(cfg, refresh).then(
    (tokens) => {
      flight.expiresAt = Date.now() + REFRESH_REPLAY_GRACE_MS;
      const timer = setTimeout(() => {
        if (refreshFlights.get(refresh) === flight) {
          refreshFlights.delete(refresh);
        }
      }, REFRESH_REPLAY_GRACE_MS);
      timer.unref?.();
      return tokens;
    },
    (error) => {
      if (refreshFlights.get(refresh) === flight) {
        refreshFlights.delete(refresh);
      }
      throw error;
    },
  );
  flight = { promise, expiresAt: Number.POSITIVE_INFINITY };
  refreshFlights.set(refresh, flight);
  return promise;
}

// sessionFromTokens builds a Session from a token response.
export function sessionFromTokens(
  tok: TokenResponse,
  prev?: Session,
  verifiedClaims?: VerifiedIdentityClaims,
): Session {
  if (!prev && !verifiedClaims) {
    throw new Error("Verified Identity claims are required for a new session");
  }
  const claims = verifiedClaims;
  return {
    access: tok.access_token,
    refresh: tok.refresh_token || prev?.refresh,
    exp: Date.now() + (tok.expires_in ?? 600) * 1000,
    user: compactSessionUser(
      prev?.user || {
        sub: claims!.sub,
        userKey: claims!.user_key || claims!.sub,
        email: claims!.email,
        name: claims!.name || claims!.preferred_username || claims!.email,
      },
    ),
  };
}

function boundedOptionalText(
  value: unknown,
  maxLength: number,
): string | undefined {
  if (typeof value !== "string") return undefined;
  const text = value.trim();
  return text ? text.slice(0, maxLength) : undefined;
}

export function compactSessionUser(user: Session["user"]): Session["user"] {
  return {
    sub: user.sub,
    userKey: boundedOptionalText(user.userKey, 160),
    email: boundedOptionalText(user.email, 320),
    name: boundedOptionalText(user.name, 160),
  };
}

export function compactSession(session: Session): Session {
  return {
    access: session.access,
    refresh: session.refresh,
    exp: session.exp,
    user: compactSessionUser(session.user),
  };
}

// safeReturnTo only allows same-origin relative paths (open-redirect guard).
export function safeReturnTo(raw: string | undefined | null): string {
  if (!raw || raw[0] !== "/") return "/";
  let candidate = raw;
  for (let pass = 0; pass < 8; pass += 1) {
    if (
      candidate[0] !== "/" ||
      candidate[1] === "/" ||
      /[\\\u0000-\u001f\u007f]/u.test(candidate)
    ) {
      return "/";
    }
    if (!candidate.includes("%")) break;
    try {
      const decoded = decodeURIComponent(candidate);
      if (decoded === candidate) break;
      candidate = decoded;
    } catch {
      return "/";
    }
    if (pass === 7) return "/";
  }
  return raw;
}
