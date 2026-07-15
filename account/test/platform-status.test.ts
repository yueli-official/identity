import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import { aggregatePlatformServices, capabilityManifestSchema, capabilityVersionSatisfies, classifyPlatformServiceFailure, evaluateCapabilityRequirements, manifestAgeSeconds, mergeCapabilityRequirements, parsePlatformManifest, platformProbeFailureStatus, readCapabilityRequirements } from '../server/utils/platform-status'
import type { PlatformServiceResult } from '../shared/types/platform'

function envelope(value = manifest()) {
  return { code: 'ok', data: { manifest: value } }
}

function manifest() {
  return {
    apiVersion: 'platform.yueli.dev/service-capability-manifest/v1',
    kind: 'ServiceCapabilityManifest',
    service: { name: 'asset', version: 'test', buildSha: 'sha', deployment: 'asset-test' },
    generatedAt: '2026-07-12T00:00:00Z',
    redaction: { policy: 'presence-only', version: '1' },
    capabilities: [{
      key: 'asset.object-storage', contractVersion: '1.0', support: 'supported',
      configuration: 'complete', enablement: 'enabled', health: 'healthy', effective: true,
      operations: ['put'], requiredConfig: [{ key: 'secret_key', state: 'present', secret: true }], links: [],
    }],
    providers: [], links: [],
  }
}

describe('platform capability BFF schema', () => {
  it('accepts a strict Manifest v1 response', () => {
    const parsed = capabilityManifestSchema.parse(manifest())
    expect(parsed.capabilities[0]?.requiredConfig[0]).toEqual({ key: 'secret_key', state: 'present', secret: true })
  })

  it('rejects undeclared config values instead of forwarding them', () => {
    const value = manifest()
    Object.assign(value.capabilities[0]!.requiredConfig[0]!, { value: 'must-not-pass' })
    expect(capabilityManifestSchema.safeParse(value).success).toBe(false)
  })

  it('rejects incompatible manifest versions', () => {
    const value = manifest()
    value.apiVersion = 'platform.yueli.dev/service-capability-manifest/v2'
    expect(capabilityManifestSchema.safeParse(value).success).toBe(false)
  })

  it('rejects a manifest returned by the wrong service', () => {
    expect(parsePlatformManifest('identity', envelope())).toBeUndefined()
  })

  it('rejects invalid snapshot and provider timestamps', () => {
    const snapshot = manifest()
    snapshot.generatedAt = 'yesterday'
    expect(capabilityManifestSchema.safeParse(snapshot).success).toBe(false)

    const runtime = manifest()
    Object.assign(runtime.capabilities[0], { lastCheckedAt: 'soon' })
    Object.assign(runtime.capabilities[0]!.requiredConfig[0], { rotatedAt: 'recently' })
    expect(capabilityManifestSchema.safeParse(runtime).success).toBe(false)
  })

  it('computes stable snapshot age and clamps future timestamps', () => {
    expect(manifestAgeSeconds('2026-07-12T00:00:00Z', Date.parse('2026-07-12T00:01:40Z'))).toBe(100)
    expect(manifestAgeSeconds('2026-07-12T00:01:40Z', Date.parse('2026-07-12T00:00:00Z'))).toBe(0)
  })

  it('keeps partial results and starts all service reads concurrently', async () => {
    const started: string[] = []
    const pending = new Map<string, (value: PlatformServiceResult) => void>()
    const result = aggregatePlatformServices(key => new Promise((resolve) => {
      started.push(key)
      pending.set(key, resolve)
    }))
    expect(started).toEqual(['identity', 'asset', 'commerce', 'notification'])
    for (const [key, resolve] of pending) {
      resolve({
        key: key as PlatformServiceResult['key'],
        status: key === 'commerce' ? 'unavailable' : 'available',
        observedAt: '2026-07-12T00:00:00Z',
        latencyMs: 1,
        ...(key === 'commerce' ? { error: { code: 'unavailable', message: 'deadline' } } : { manifest: manifest() }),
      })
    }
    expect((await result).map(item => item.status)).toEqual(['available', 'available', 'unavailable', 'available'])
  })

  it('preserves forbidden, timeout, rate-limit, and probe failure semantics', () => {
    expect(classifyPlatformServiceFailure(Object.assign(new Error('denied'), { statusCode: 403 })).status).toBe('forbidden')
    expect(classifyPlatformServiceFailure(Object.assign(new Error('deadline'), { statusCode: 504 })).status).toBe('unavailable')
    expect(platformProbeFailureStatus(Object.assign(new Error('limited'), { response: { status: 429 } }))).toBe(429)
    expect(platformProbeFailureStatus(new Error('audit failed'))).toBe(502)
  })

  it('evaluates application requirements against effective capabilities and versions', () => {
    const assetManifest = manifest()
    const services: PlatformServiceResult[] = [{
      key: 'asset', status: 'available', observedAt: '2026-07-12T00:00:00Z', latencyMs: 1, manifest: assetManifest,
    }]
    const [application] = evaluateCapabilityRequirements([{
      site: 'blog-ai', productType: 'blog', brand: 'AI Blog',
      capabilities: { 'asset.object-storage': '>=1.0', 'identity.oidc': '>=1.0' },
    }], services)
    expect(application?.satisfied).toBe(false)
    expect(application?.requirements).toEqual([
      { key: 'asset.object-storage', constraint: '>=1.0', actualVersion: '1.0', satisfied: true },
      { key: 'identity.oidc', constraint: '>=1.0', reason: 'service_unavailable', actualVersion: undefined, satisfied: false },
    ])
  })

  it('distinguishes configuration, enablement, health, and contract version gaps', () => {
    const states = [
      { constraint: '>=2.0', reason: 'version_incompatible' },
      { constraint: '>=1.0', configuration: 'partial', reason: 'configuration_incomplete' },
      { constraint: '>=1.0', enablement: 'disabled', reason: 'disabled' },
      { constraint: '>=1.0', effective: false, health: 'unhealthy', reason: 'unhealthy' },
    ] as const
    for (const state of states) {
      const value = manifest()
      Object.assign(value.capabilities[0]!, state)
      const [application] = evaluateCapabilityRequirements([{
        site: 'docs-main', productType: 'docs', brand: 'Docs', capabilities: { 'asset.object-storage': state.constraint },
      }], [{ key: 'asset', status: 'available', observedAt: value.generatedAt, latencyMs: 1, manifest: value }])
      expect(application?.requirements[0]?.reason).toBe(state.reason)
    }
  })

  it('supports exact and minimum capability contract versions', () => {
    expect(capabilityVersionSatisfies('1.2', '>=1.1')).toBe(true)
    expect(capabilityVersionSatisfies('1.0', '>=1.0.1')).toBe(false)
    expect(capabilityVersionSatisfies('2.0.0', '=2.0')).toBe(true)
    expect(capabilityVersionSatisfies('2.1', '2.0')).toBe(false)
    expect(capabilityVersionSatisfies('latest', '>=1.0')).toBe(false)
  })

  it('merges attached composition requirements deterministically and rejects conflicts', () => {
    const blog = { site: 'blog-ai', productType: 'blog', brand: 'AI Blog', capabilities: { 'identity.oidc': '>=1.0' } }
    const docs = { site: 'docs-main', productType: 'docs', brand: 'Docs', capabilities: { 'identity.oidc': '>=1.0' } }
    expect(mergeCapabilityRequirements([[docs], [blog], [blog]])).toEqual([blog, docs])
    expect(() => mergeCapabilityRequirements([[blog], [{ ...blog, capabilities: { 'identity.oidc': '>=2.0' } }]])).toThrow('conflicting')
  })

  it('loads strict attach registrations from the Core control directory', async () => {
    const directory = await mkdtemp(join(tmpdir(), 'account-compositions-'))
    const base = [{ site: 'blog-ai', productType: 'blog', brand: 'AI Blog', capabilities: { 'identity.oidc': '>=1.0' } }]
    const attached = [{ site: 'docs-main', productType: 'docs', brand: 'Docs', capabilities: { 'asset.object-storage': '>=1.0' } }]
    await writeFile(join(directory, 'docs.json'), JSON.stringify(attached))
    vi.stubGlobal('useRuntimeConfig', () => ({
      platformCapabilityRequirementsB64: Buffer.from(JSON.stringify(base)).toString('base64'),
      platformCompositionDir: directory,
    }))
    try {
      expect(await readCapabilityRequirements({} as never)).toEqual([...base, ...attached])
    } finally {
      vi.unstubAllGlobals()
      await rm(directory, { recursive: true, force: true })
    }
  })
})
