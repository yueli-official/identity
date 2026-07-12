export interface AssetAdminStats {
  assets: number
  publicAssets: number
  privateAssets: number
  sites: number
  profiles: number
  activeGrants: number
}

export interface AssetSite {
  siteKey: string
  name: string
  defaultStorageBackend: string
  enabled: boolean
  assetCount: number
  profileCount: number
  variantCount: number
}

export interface AssetStorageBackend {
  name: string
  type?: string
  enabled: boolean
  managed: boolean
  isDefault: boolean
  healthy: boolean
  secretVersion: number
  secretRotatedAt?: string
  lastHealthOk?: boolean
  lastHealthError?: string
  lastHealthCheckedAt?: string
  assetCount: number
  siteCount: number
  profileCount: number
  error?: string
}

export interface AssetStorageBackendDetail {
  name: string
  type: string
  enabled: boolean
  endpoint: string
  region: string
  bucketPublic: string
  bucketPrivate: string
  accessKey: string
  hasSecretKey: boolean
  publicBaseUrl: string
  pathStyle: boolean
  useSsl: boolean
  secretVersion: number
  secretRotatedAt?: string
  lastHealthOk?: boolean
  lastHealthError?: string
  lastHealthCheckedAt?: string
}

export interface AssetStorageBackendForm {
  name: string
  type: string
  enabled: boolean
  endpoint: string
  region: string
  bucketPublic: string
  bucketPrivate: string
  accessKey: string
  secretKey: string
  publicBaseUrl: string
  pathStyle: boolean
  useSsl: boolean
}

export interface AssetStorageBackendEvent {
  id: string
  backendName: string
  eventType: string
  status: 'ok' | 'error' | string
  actor: string
  message: string
  metadata: string
  createdAt: string
}

export interface AssetProfile {
  siteKey: string
  profileKey: string
  purpose: string
  storageBackend: string
  allowedExt: string
  maxSizeBytes: number
  defaultVisibility: string
  defaultDeliveryPolicy: string
  keepOriginal: boolean
  assetCount: number
  variantCount: number
}

export interface AssetVariant {
  id: string
  siteKey: string
  profileKey: string
  variantKey: string
  width: number
  height: number
  mode: string
  format: string
  quality: number
  version: number
  enabled: boolean
}

export interface AssetItem {
  id: string
  visibility: string
  filename: string
  mime: string
  size: number
  width?: number
  height?: number
  category: string
  spaceKey: string
  siteKey: string
  profileKey: string
  deliveryPolicy: string
  storageBackend: string
  refCount: number
  cdnUrl?: string
  createdAt: string
}

export interface AssetSpaceUsage {
  spaceKey: string
  assetCount: number
  totalBytes: number
}

export interface AssetReference {
  id: string
  assetId: string
  siteKey: string
  refType: string
  refId: string
  refLabel: string
  refUrl: string
  createdByService: string
  createdAt: string
}

export interface AssetSweepResult {
  backend: string
  removed: number
  skipped: boolean
  error?: string
}

export interface AssetOperationError {
  id: string
  filename: string
  error: string
}

export interface AssetPruneResult {
  candidates: number
  deleted: number
  items: AssetItem[]
  errors?: AssetOperationError[]
  task?: AssetMaintenanceTask
}

export interface AssetPruneForm {
  olderThanDays: number
  limit: number
}

export interface AssetOrphanObjectForm {
  olderThanDays: number
  limit: number
  backend: string
}

export interface AssetStorageMigrationForm {
  sourceBackend: string
  targetBackend: string
  limit: number
}

export interface AssetBatchRebuildResult {
  candidates: number
  rebuilt: number
  generated: number
  items: AssetItem[]
  errors?: AssetOperationError[]
  task?: AssetMaintenanceTask
}

export interface AssetOrphanObjectItem {
  key: string
  size: number
  modTime?: string
}

export interface AssetOrphanObjectBackend {
  backend: string
  scanned: number
  expected: number
  orphans: number
  deleted: number
  skipped: boolean
  error?: string
  items: AssetOrphanObjectItem[]
  errors?: { key: string, error: string }[]
}

export interface AssetOrphanObjectResult {
  items: AssetOrphanObjectBackend[]
  orphans: number
  deleted: number
  task?: AssetMaintenanceTask
}

export interface AssetStorageMigrationResult {
  candidates: number
  migrated: number
  items: AssetItem[]
  errors?: AssetOperationError[]
  task?: AssetMaintenanceTask
}

export interface AssetMaintenanceTask {
  id: string
  taskType: string
  status: 'queued' | 'running' | 'retrying' | 'paused' | 'completed' | 'failed' | 'cancelled'
  dryRun: boolean
  summary: string
  payload?: string
  result?: string
  attempts: number
  maxAttempts: number
  error?: string
  nextRunAt?: string
  lockedAt?: string
  lockedBy?: string
  startedAt?: string
  finishedAt?: string
  createdAt: string
}

export interface AssetMaintenanceTaskResult {
  candidates?: number
  rebuilt?: number
  generated?: number
  errors?: Array<{ id?: string, filename?: string, error: string }>
  lastAsset?: string
}

export interface AssetGrant {
  id: string
  assetId: string
  variantKey: string
  siteKey: string
  subjectId: string
  policy: string
  purpose: string
  expiresAt: string
  maxUses: number
  usedCount: number
  firstUsedAt?: string
  lastUsedAt?: string
  revokedAt?: string
  revokedBy?: string
  createdByService: string
  reason: string
  createdAt: string
}

export interface AssetGrantForm {
  variantKey: string
  policy: string
  subjectId: string
  expiresIn: number
  maxUses: number
  reason: string
}

export interface CreatedAssetGrant {
  grantId: string
  url: string
  expiresAt: string
}

export type AssetAdminSection = 'library' | 'sites' | 'profiles' | 'storage' | 'maintenance' | 'grants'
