export type RuntimeConfiguration = 'missing' | 'partial' | 'complete'
export type RuntimeEnablement = 'enabled' | 'disabled'
export type RuntimeHealth = 'unknown' | 'healthy' | 'degraded' | 'unhealthy'

export interface ConfigField {
  key: string
  state: 'present' | 'missing'
  secret: boolean
  version?: string
  rotatedAt?: string
}

export interface CapabilityItem {
  key: string
  contractVersion: string
  support: 'supported' | 'unsupported'
  configuration: RuntimeConfiguration
  enablement: RuntimeEnablement
  health: RuntimeHealth
  effective: boolean
  adapter?: string
  providerInstance?: string
  operations: string[]
  requiredConfig: ConfigField[]
  lastCheckedAt?: string
  links: Array<{ rel: string, href: string }>
}

export interface ProviderItem extends Omit<CapabilityItem, 'contractVersion' | 'support' | 'providerInstance'> {
  adapter: string
  registered: boolean
  capabilityKeys: string[]
  verifiedCompatibility: string[]
  mode?: string
}

export interface CapabilityManifest {
  apiVersion: string
  kind: string
  service: { name: string, version: string, buildSha: string, deployment: string }
  generatedAt: string
  redaction: { policy: string, version: string }
  capabilities: CapabilityItem[]
  providers: ProviderItem[]
  links: Array<{ rel: string, href: string }>
}

export type PlatformServiceKey = 'identity' | 'asset' | 'commerce' | 'notification'

export interface PlatformServiceResult {
  key: PlatformServiceKey
  status: 'available' | 'unavailable' | 'forbidden' | 'incompatible'
  observedAt: string
  latencyMs: number
  ageSeconds?: number
  manifest?: CapabilityManifest
  error?: { code: string, message: string }
}

export interface PlatformStatusResponse {
  observedAt: string
  environment: string
  catalogFingerprint: string
  summary: { total: number, available: number, effectiveCapabilities: number, capabilityIssues: number }
  services: PlatformServiceResult[]
}
