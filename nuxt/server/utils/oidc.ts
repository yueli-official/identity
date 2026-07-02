/// <reference types="node" />
// OIDC BFF helpers for consumer sites. No external
// OIDC/jose lib — Node's crypto does PKCE (sha256) + a sealed AES-256-GCM cookie.
// (The node reference makes node:crypto + Buffer resolve under each app's
// vue-tsc, which checks this layer's server code with the client tsconfig.)
// Tokens live only server-side; the browser only ever holds the sealed cookie.
import { randomBytes, createHash, createCipheriv, createDecipheriv } from 'node:crypto'
import type { H3Event } from 'h3'

export interface OidcCfg {
  clientId: string
  redirectUri: string
  scopes: string
  authorizeEndpoint: string
  tokenEndpoint: string
  endSessionEndpoint: string
  downstreamBase: string
  sealSecret: string
}

export interface Session {
  access: string
  refresh?: string
  exp: number // epoch ms
  user: { sub: string; email?: string; name?: string; avatar?: string; roles?: string[] }
}

export const SESSION_COOKIE = 'rs_session'
export const TX_COOKIE = 'rs_oidc_tx'

export function oidcConfig(event: H3Event): OidcCfg {
  const rc = useRuntimeConfig(event)
  const issuer = (rc.public.oidcIssuer as string).replace(/\/$/, '')
  return {
    clientId: rc.public.oidcClientId as string,
    redirectUri: rc.public.oidcRedirectUri as string,
    scopes: (rc.public.oidcScopes as string) || 'openid profile email roles offline_access',
    authorizeEndpoint: issuer + '/oauth2/authorize',
    tokenEndpoint: issuer + '/oauth2/token',
    endSessionEndpoint: issuer + '/oauth2/end_session',
    downstreamBase: (rc.downstreamBase as string).replace(/\/$/, ''),
    sealSecret: rc.sealSecret as string
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

// decodeJwt reads a JWT's payload WITHOUT verifying it. Only used for display
// claims (sub/email/name) on tokens we just received over a trusted channel.
export function decodeJwt(token: string): Record<string, any> {
  try {
    return JSON.parse(Buffer.from(token.split('.')[1] ?? '', 'base64url').toString('utf8'))
  } catch {
    return {}
  }
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
      code_verifier: verifier
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
      client_id: cfg.clientId
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
export function sessionFromTokens(tok: TokenResponse, prev?: Session): Session {
  const claims = decodeJwt(tok.id_token || tok.access_token)
  return {
    access: tok.access_token,
    refresh: tok.refresh_token || prev?.refresh,
    exp: Date.now() + (tok.expires_in ?? 600) * 1000,
    user: prev?.user || {
      sub: claims.sub,
      email: claims.email,
      name: claims.name || claims.preferred_username || claims.email,
      avatar: claims.picture,
      roles: Array.isArray(claims.roles) ? claims.roles : []
    }
  }
}

// safeReturnTo only allows same-origin relative paths (open-redirect guard).
export function safeReturnTo(raw: string | undefined | null): string {
  if (!raw || raw[0] !== '/') return '/'
  if (/[\\\r\n\t]/.test(raw)) return '/'
  if (raw[1] === '/') return '/'
  return raw
}
