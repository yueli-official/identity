import { z } from 'zod'
import type { H3Event } from 'h3'
import { readdir, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import type { ApplicationCapabilityStatus, CapabilityItem, CapabilityManifest, CapabilityRequirementResult, PlatformServiceKey, PlatformServiceResult, ProviderItem, SiteCapabilityRequirements } from '../../shared/types/platform'

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

const manifestResponseSchema = z.object({
  manifest: capabilityManifestSchema,
}).strict()

const providerEnvelopeSchema = z.object({
  code: z.literal('ok'),
  data: z.object({
    provider: providerItemSchema,
  }).strict(),
}).strict()

const providerResponseSchema = z.object({
  provider: providerItemSchema,
}).strict()

const sessionSchema = z.object({
  roles: z.array(z.string()),
})

const sessionEnvelopeSchema = z.object({
  code: z.literal('ok'),
  data: sessionSchema,
}).strict()

const capabilityConstraintSchema = z.string().regex(/^(?:>=|=)?\d+\.\d+(?:\.\d+)?$/)
const siteCapabilityRequirementsSchema = z.array(z.object({
  site: z.string().regex(/^[a-z0-9][a-z0-9-]*$/),
  productType: z.string().regex(/^[a-z0-9][a-z0-9-]*$/),
  brand: z.string(),
  capabilities: z.record(z.string().regex(/^[a-z][a-z0-9-]*\.[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/), capabilityConstraintSchema),
}).strict())

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
    const roles = parsePlatformAdminRoles(response)
    if (!roles.includes('admin')) {
      throw createError({ statusCode: 403, statusMessage: 'Admin role required' })
    }
  } catch (error) {
    const status = responseStatus(error)
    if (status === 401) throw createError({ statusCode: 401, statusMessage: 'Authentication required' })
    if (status === 403) throw createError({ statusCode: 403, statusMessage: 'Admin role required' })
    throw createError({ statusCode: 503, statusMessage: 'Identity service unavailable' })
  }
}

export function parsePlatformAdminRoles(response: unknown): string[] {
  const direct = sessionSchema.safeParse(response)
  if (direct.success) return direct.data.roles
  return sessionEnvelopeSchema.parse(response).data.roles
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
  const direct = manifestResponseSchema.safeParse(response)
  if (direct.success) {
    return direct.data.manifest.service.name === key
      ? direct.data.manifest as CapabilityManifest
      : undefined
  }
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

export async function readCapabilityRequirements(event: H3Event): Promise<SiteCapabilityRequirements[]> {
  const config = useRuntimeConfig(event)
  const sources: SiteCapabilityRequirements[][] = []
  try {
    sources.push(siteCapabilityRequirementsSchema.parse(JSON.parse(Buffer.from(config.platformCapabilityRequirementsB64, 'base64').toString('utf8'))))
    if (config.platformCompositionDir) {
      let entries: string[] = []
      try {
        entries = (await readdir(config.platformCompositionDir, { withFileTypes: true }))
          .filter(entry => entry.isFile() && entry.name.endsWith('.json'))
          .map(entry => entry.name)
          .sort()
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error
      }
      if (entries.length > 128) throw new Error('too many composition registrations')
      for (const filename of entries) {
        const data = await readFile(join(config.platformCompositionDir, filename))
        if (data.length > 1 << 20) throw new Error('composition registration exceeds 1 MiB')
        sources.push(siteCapabilityRequirementsSchema.parse(JSON.parse(data.toString('utf8'))))
      }
    }
    return mergeCapabilityRequirements(sources)
  } catch {
    throw createError({ statusCode: 503, statusMessage: 'Platform capability requirements are invalid' })
  }
}

export function mergeCapabilityRequirements(sources: SiteCapabilityRequirements[][]): SiteCapabilityRequirements[] {
  const merged = new Map<string, SiteCapabilityRequirements>()
  for (const source of sources) {
    for (const application of source) {
      const existing = merged.get(application.site)
      if (existing && JSON.stringify(existing) !== JSON.stringify(application)) {
        throw new Error(`conflicting capability requirements for ${application.site}`)
      }
      merged.set(application.site, application)
    }
  }
  return [...merged.values()].sort((left, right) => left.site.localeCompare(right.site))
}

export function evaluateCapabilityRequirements(requirements: SiteCapabilityRequirements[], services: PlatformServiceResult[]): ApplicationCapabilityStatus[] {
  const serviceMap = new Map(services.map(service => [service.key, service]))
  return requirements.map(application => {
    const results = Object.entries(application.capabilities).sort(([left], [right]) => left.localeCompare(right))
      .map(([key, constraint]) => evaluateCapabilityRequirement(key, constraint, serviceMap))
    return { ...application, satisfied: results.every(result => result.satisfied), requirements: results }
  })
}

function evaluateCapabilityRequirement(key: string, constraint: string, services: Map<PlatformServiceKey, PlatformServiceResult>): CapabilityRequirementResult {
  const serviceKey = key.split('.', 1)[0] as PlatformServiceKey
  const service = services.get(serviceKey)
  if (!service || service.status !== 'available' || !service.manifest) return gap(key, constraint, 'service_unavailable')
  const capability = service.manifest.capabilities.find(item => item.key === key)
  if (!capability) return gap(key, constraint, 'capability_missing')
  if (capability.support !== 'supported') return gap(key, constraint, 'unsupported', capability)
  if (!capabilityVersionSatisfies(capability.contractVersion, constraint)) return gap(key, constraint, 'version_incompatible', capability)
  if (capability.configuration !== 'complete') return gap(key, constraint, 'configuration_incomplete', capability)
  if (capability.enablement !== 'enabled') return gap(key, constraint, 'disabled', capability)
  if (!capability.effective) return gap(key, constraint, 'unhealthy', capability)
  return { key, constraint, actualVersion: capability.contractVersion, satisfied: true }
}

function gap(key: string, constraint: string, reason: CapabilityRequirementResult['reason'], capability?: CapabilityItem): CapabilityRequirementResult {
  return { key, constraint, reason, actualVersion: capability?.contractVersion, satisfied: false }
}

export function capabilityVersionSatisfies(actual: string, constraint: string): boolean {
  const actualVersion = parseCapabilityVersion(actual)
  const operator = constraint.startsWith('>=') ? '>=' : '='
  const requiredVersion = parseCapabilityVersion(constraint.replace(/^(?:>=|=)/, ''))
  if (!actualVersion || !requiredVersion) return false
  const comparison = actualVersion.findIndex((part, index) => part !== requiredVersion[index])
  if (comparison === -1) return true
  return operator === '>=' && actualVersion[comparison]! > requiredVersion[comparison]!
}

function parseCapabilityVersion(value: string): [number, number, number] | undefined {
  if (!/^\d+\.\d+(?:\.\d+)?$/.test(value)) return undefined
  const parts = value.split('.').map(Number)
  return [parts[0]!, parts[1]!, parts[2] ?? 0]
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
  const parsedProvider = parsePlatformProviderResponse(response)
  if (!parsedProvider) {
    throw createError({ statusCode: 502, statusMessage: 'Provider response does not match Manifest v1' })
  }
  return parsedProvider
}

export function parsePlatformProviderResponse(response: unknown): ProviderItem | undefined {
  const direct = providerResponseSchema.safeParse(response)
  if (direct.success) return direct.data.provider as ProviderItem
  const envelope = providerEnvelopeSchema.safeParse(response)
  return envelope.success ? envelope.data.data.provider as ProviderItem : undefined
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
