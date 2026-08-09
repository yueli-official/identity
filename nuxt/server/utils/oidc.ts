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
import type { H3Event } from 'h3'

export interface OidcCfg {
  issuer: string
  clientId: string
  clientSecret: string
  redirectUri: string
  postLogoutRedirectUri: string
  scopes: string
  authorizeEndpoint: string
  tokenEndpoint: string
  jwksEndpoint: string
  endSessionEndpoint: string
  downstreamBase: string
  sealSecret: string
  cookieSecure: boolean
}

export interface Session {
  access: string
  refresh?: string
  exp: number // epoch ms
  user: { sub: string; userKey?: string; email?: string; name?: string; avatar?: string; roles?: string[] }
}

export const SESSION_COOKIE = 'rs_session'
export const TX_COOKIE = 'rs_oidc_tx'

export function oidcConfig(event: H3Event): OidcCfg {
  const rc = useRuntimeConfig(event)
  const issuer = (rc.public.oidcIssuer as string).replace(/\/$/, '')
  return {
    issuer,
    clientId: rc.public.oidcClientId as string,
    clientSecret: rc.oidcClientSecret as string,
    redirectUri: rc.public.oidcRedirectUri as string,
    postLogoutRedirectUri: rc.public.oidcPostLogoutRedirectUri as string,
    scopes: (rc.public.oidcScopes as string) || 'openid profile email roles offline_access',
    authorizeEndpoint: issuer + '/oauth2/authorize',
    tokenEndpoint: issuer + '/oauth2/token',
    jwksEndpoint: issuer + '/oauth2/jwks.json',
    endSessionEndpoint: issuer + '/oauth2/end_session',
    downstreamBase: (rc.downstreamBase as string).replace(/\/$/, ''),
    sealSecret: rc.sealSecret as string,
    cookieSecure: Boolean(rc.authCookieSecure),
  }
}

function b64url(b: Buffer) {
  return b.toString('base64url')
}

// PKCE (S256): a random verifier + its sha256 challenge.
export function pkce() {
  const verifier = b64url(randomBytes(32))
  const challenge = b64url(createHash('sha256').update(verifier).digest())
  return { verifier, challenge }
}

export function randomToken(n = 16) {
  return b64url(randomBytes(n))
}

function keyOf(secret: string) {
  return createHash('sha256').update(secret).digest() // 32 bytes for AES-256
}

// seal encrypts+authenticates a JSON value into a compact base64url token.
export function seal(data: unknown, secret: string): string {
  const iv = randomBytes(12)
  const c = createCipheriv('aes-256-gcm', keyOf(secret), iv)
  const enc = Buffer.concat([c.update(JSON.stringify(data), 'utf8'), c.final()])
  return b64url(Buffer.concat([iv, c.getAuthTag(), enc]))
}

export function unseal<T>(token: string | undefined, secret: string): T | null {
  if (!token) return null
  try {
    const buf = Buffer.from(token, 'base64url')
    const iv = buf.subarray(0, 12)
    const tag = buf.subarray(12, 28)
    const enc = buf.subarray(28)
    const d = createDecipheriv('aes-256-gcm', keyOf(secret), iv)
    d.setAuthTag(tag)
    const dec = Buffer.concat([d.update(enc), d.final()]).toString('utf8')
    return JSON.parse(dec) as T
  } catch {
    return null
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
  if (parts.length !== 3) throw new Error("Identity returned an invalid ID Token");
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
  access_token: string
  refresh_token?: string
  id_token?: string
  expires_in?: number
}

export async function exchangeCode(cfg: OidcCfg, code: string, verifier: string): Promise<TokenResponse> {
  return await $fetch<TokenResponse>(cfg.tokenEndpoint, {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      code,
      redirect_uri: cfg.redirectUri,
      client_id: cfg.clientId,
      code_verifier: verifier,
      ...(cfg.clientSecret ? { client_secret: cfg.clientSecret } : {})
    }).toString()
  })
}

export async function refreshTokens(cfg: OidcCfg, refresh: string): Promise<TokenResponse> {
  return await $fetch<TokenResponse>(cfg.tokenEndpoint, {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: refresh,
      client_id: cfg.clientId,
      ...(cfg.clientSecret ? { client_secret: cfg.clientSecret } : {})
    }).toString()
  })
}

// In-flight refreshes keyed by the refresh token, so concurrent requests on one
// session (all reading the same sealed cookie near expiry) share a single token
// grant instead of each POSTing /token. Without this, refresh-token ROTATION
// makes the parallel grants invalidate each other. The dedup is per Nitro
// instance (the cookie is stateless, so cross-instance dedup is out of scope).
const inflightRefresh = new Map<string, Promise<TokenResponse>>()

export function refreshSingleFlight(cfg: OidcCfg, refresh: string): Promise<TokenResponse> {
  const existing = inflightRefresh.get(refresh)
  if (existing) return existing
  const p = refreshTokens(cfg, refresh).finally(() => inflightRefresh.delete(refresh))
  inflightRefresh.set(refresh, p)
  return p
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
    user: prev?.user || {
      sub: claims!.sub,
      userKey: claims!.user_key || claims!.sub,
      email: claims!.email,
      name:
        claims!.name || claims!.preferred_username || claims!.email,
      avatar: claims!.picture,
      roles: Array.isArray(claims!.roles) ? claims!.roles : [],
    }
  }
}

// safeReturnTo only allows same-origin relative paths (open-redirect guard).
export function safeReturnTo(raw: string | undefined | null): string {
  if (!raw || raw[0] !== '/') return '/'
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
  return raw
}
