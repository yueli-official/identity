import { z } from 'zod'
import type { H3Event } from 'h3'
import type { CapabilityManifest, PlatformServiceKey, PlatformServiceResult, ProviderItem } from '../../shared/types/platform'

export const platformServiceKeys = ['identity', 'asset', 'commerce', 'notification'] as const

const linkSchema = z.object({ rel: z.string(), href: z.string() }).strict()
const configFieldSchema = z.object({
  key: z.string(),
  state: z.enum(['present', 'missing']),
  secret: z.boolean(),
  version: z.string().optional(),
  rotatedAt: z.iso.datetime({ offset: true }).optional(),
}).strict()

const runtimeStateSchema = z.object({
  key: z.string(),
  configuration: z.enum(['missing', 'partial', 'complete']),
  enablement: z.enum(['enabled', 'disabled']),
  health: z.enum(['unknown', 'healthy', 'degraded', 'unhealthy']),
  effective: z.boolean(),
  adapter: z.string().optional(),
  providerInstance: z.string().optional(),
  operations: z.array(z.string()),
  requiredConfig: z.array(configFieldSchema),
  lastCheckedAt: z.iso.datetime({ offset: true }).optional(),
  links: z.array(linkSchema),
}).strict()

const providerItemSchema = runtimeStateSchema.omit({ providerInstance: true }).extend({
  adapter: z.string(),
  registered: z.boolean(),
  capabilityKeys: z.array(z.string()),
  verifiedCompatibility: z.array(z.string()),
  mode: z.string().optional(),
}).strict()

export const capabilityManifestSchema = z.object({
  apiVersion: z.literal('platform.yueli.dev/service-capability-manifest/v1'),
  kind: z.literal('ServiceCapabilityManifest'),
  service: z.object({
    name: z.string(),
    version: z.string(),
    buildSha: z.string(),
    deployment: z.string(),
  }).strict(),
  generatedAt: z.iso.datetime({ offset: true }),
  redaction: z.object({ policy: z.string(), version: z.string() }).strict(),
  capabilities: z.array(runtimeStateSchema.extend({ contractVersion: z.string(), support: z.enum(['supported', 'unsupported']) }).strict()),
  providers: z.array(providerItemSchema),
  links: z.array(linkSchema),
}).strict()

const manifestEnvelopeSchema = z.object({
  code: z.literal('ok'),
  data: z.object({ manifest: capabilityManifestSchema }).strict(),
}).strict()

const providerEnvelopeSchema = z.object({
  code: z.literal('ok'),
  data: z.object({
    provider: providerItemSchema,
  }).strict(),
}).strict()

const sessionEnvelopeSchema = z.object({
  code: z.literal('ok'),
  data: z.object({ roles: z.array(z.string()) }).strict(),
}).strict()

function identityHeaders(event: H3Event): Record<string, string> {
  const cookie = getRequestHeader(event, 'cookie')
  return cookie ? { cookie } : {}
}

export async function requirePlatformAdmin(event: H3Event) {
  const base = useRuntimeConfig(event).apiBase.replace(/\/$/, '')
  try {
    const response = await $fetch(`${base}/api/v1/session/me`, {
      headers: identityHeaders(event),
      signal: AbortSignal.timeout(2500),
    })
    const session = sessionEnvelopeSchema.parse(response)
    if (!session.data.roles.includes('admin')) {
      throw createError({ statusCode: 403, statusMessage: 'Admin role required' })
    }
  } catch (error) {
    const status = responseStatus(error)
    if (status === 401) throw createError({ statusCode: 401, statusMessage: 'Authentication required' })
    if (status === 403) throw createError({ statusCode: 403, statusMessage: 'Admin role required' })
    throw createError({ statusCode: 503, statusMessage: 'Identity service unavailable' })
  }
}

export async function fetchPlatformService(event: H3Event, key: PlatformServiceKey): Promise<PlatformServiceResult> {
  const startedAt = performance.now()
  const observedAt = new Date().toISOString()
  const base = useRuntimeConfig(event).apiBase.replace(/\/$/, '')
  const path = key === 'identity'
    ? '/api/v1/admin/capabilities'
    : `/api/v1/admin/platform-proxy/${key}/capabilities`
  try {
    const response = await $fetch(base + path, {
      headers: identityHeaders(event),
      signal: AbortSignal.timeout(2500),
    })
    const manifest = parsePlatformManifest(key, response)
    if (!manifest) {
      return failure(key, 'incompatible', observedAt, startedAt, 'schema_incompatible', 'Capability response does not match Manifest v1')
    }
    return {
      key,
      status: 'available',
      observedAt,
      latencyMs: elapsed(startedAt),
      ageSeconds: manifestAgeSeconds(manifest.generatedAt),
      manifest,
    }
  } catch (error) {
    const classified = classifyPlatformServiceFailure(error)
    return failure(key, classified.status, observedAt, startedAt, classified.code, classified.message)
  }
}

export function parsePlatformManifest(key: PlatformServiceKey, response: unknown): CapabilityManifest | undefined {
  const envelope = manifestEnvelopeSchema.safeParse(response)
  if (!envelope.success || envelope.data.data.manifest.service.name !== key) return undefined
  return envelope.data.data.manifest as CapabilityManifest
}

export function manifestAgeSeconds(generatedAt: string, now = Date.now()): number {
  return Math.max(0, Math.floor((now - Date.parse(generatedAt)) / 1000))
}

export async function aggregatePlatformServices(fetcher: (key: PlatformServiceKey) => Promise<PlatformServiceResult>): Promise<PlatformServiceResult[]> {
  return Promise.all(platformServiceKeys.map(key => fetcher(key)))
}

export function classifyPlatformServiceFailure(error: unknown): { status: 'forbidden' | 'unavailable', code: string, message: string } {
  const status = responseStatus(error)
  if (status === 401 || status === 403) {
    return { status: 'forbidden', code: 'forbidden', message: 'Capability access was denied' }
  }
  return { status: 'unavailable', code: 'unavailable', message: 'Service did not return a valid response before the deadline' }
}

export function platformProbeFailureStatus(error: unknown): 429 | 502 {
  return responseStatus(error) === 429 ? 429 : 502
}

export async function probePlatformProvider(event: H3Event, key: PlatformServiceKey, provider: string): Promise<ProviderItem> {
  if (!platformServiceKeys.includes(key) || !/^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/.test(provider)) {
    throw createError({ statusCode: 404, statusMessage: 'Unknown platform provider' })
  }
  const base = useRuntimeConfig(event).apiBase.replace(/\/$/, '')
  const path = key === 'identity'
    ? `/api/v1/admin/providers/${encodeURIComponent(provider)}/health-check`
    : `/api/v1/admin/platform-proxy/${key}/providers/${encodeURIComponent(provider)}/health-check`
  let response: unknown
  try {
    response = await $fetch(base + path, {
      method: 'POST',
      headers: identityHeaders(event),
      signal: AbortSignal.timeout(11000),
    })
  } catch (error) {
    const statusCode = platformProbeFailureStatus(error)
    throw createError({ statusCode, statusMessage: statusCode === 429 ? 'Provider probe rate limited' : 'Provider probe failed' })
  }
  const parsed = providerEnvelopeSchema.safeParse(response)
  if (!parsed.success) {
    throw createError({ statusCode: 502, statusMessage: 'Provider response does not match Manifest v1' })
  }
  return parsed.data.data.provider as ProviderItem
}

function failure(key: PlatformServiceKey, status: PlatformServiceResult['status'], observedAt: string, startedAt: number, code: string, message: string): PlatformServiceResult {
  return { key, status, observedAt, latencyMs: elapsed(startedAt), error: { code, message } }
}

function elapsed(startedAt: number) {
  return Math.max(0, Math.round(performance.now() - startedAt))
}

function responseStatus(error: unknown) {
  if (!(error instanceof Error)) return 0
  const candidate = error as { statusCode?: number, response?: { status?: number } }
  return candidate.statusCode || candidate.response?.status || 0
}
