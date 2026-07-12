<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import {
  ManageCollectionToolbar,
  ManageEmpty,
  ManageHeader,
  ManageRowShell,
  ManageViewToggle,
  SkeletonList
} from '@platform/manage/components'
import {
  manageCollectionQueryFingerprint,
  serializeManageCollectionQuery,
  type ManageCollectionDefinition
} from '@platform/manage/collection'
import { useManageCollectionState } from '@platform/manage/use-manage-collection-state'
import { useManageSelection } from '@platform/manage/use-manage-selection'
import { useMinLoading } from '@platform/ui/use-min-loading'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '资源管理 · 控制台' })

const { call } = useApi()
const toast = useToast()
const route = useRoute()
const router = useRouter()
const ALL = '__all__'

interface Stats {
  assets: number
  publicAssets: number
  privateAssets: number
  sites: number
  profiles: number
  activeGrants: number
}
interface Site {
  siteKey: string
  name: string
  defaultStorageBackend: string
  enabled: boolean
  assetCount: number
  profileCount: number
  variantCount: number
}
interface StorageBackend {
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
interface StorageBackendDetail {
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
interface StorageBackendEvent {
  id: string
  backendName: string
  eventType: string
  status: 'ok' | 'error' | string
  actor: string
  message: string
  metadata: string
  createdAt: string
}
interface Profile {
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
interface Variant {
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
interface AssetItem {
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
interface AssetSpaceUsage {
  spaceKey: string
  assetCount: number
  totalBytes: number
}
interface AssetReference {
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
interface SweepResult {
  backend: string
  removed: number
  skipped: boolean
  error?: string
}
interface PruneError {
  id: string
  filename: string
  error: string
}
interface PruneResult {
  candidates: number
  deleted: number
  items: AssetItem[]
  errors?: PruneError[]
  task?: MaintenanceTask
}
interface BatchRebuildResult {
  candidates: number
  rebuilt: number
  generated: number
  items: AssetItem[]
  errors?: PruneError[]
  task?: MaintenanceTask
}
interface OrphanObjectItem {
  key: string
  size: number
  modTime?: string
}
interface OrphanObjectBackend {
  backend: string
  scanned: number
  expected: number
  orphans: number
  deleted: number
  skipped: boolean
  error?: string
  items: OrphanObjectItem[]
  errors?: { key: string, error: string }[]
}
interface OrphanObjectResult {
  items: OrphanObjectBackend[]
  orphans: number
  deleted: number
  task?: MaintenanceTask
}
interface StorageMigrationResult {
  candidates: number
  migrated: number
  items: AssetItem[]
  errors?: PruneError[]
  task?: MaintenanceTask
}
interface MaintenanceTask {
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
interface MaintenanceTaskError {
  id?: string
  filename?: string
  error: string
}
interface MaintenanceTaskResult {
  candidates?: number
  rebuilt?: number
  generated?: number
  errors?: MaintenanceTaskError[]
  lastAsset?: string
}
interface Grant {
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
interface CreatedGrant {
  grantId: string
  url: string
  expiresAt: string
}

const mounted = ref(false)
const loading = ref(true)
type AssetAdminSection = 'library' | 'sites' | 'profiles' | 'storage' | 'maintenance' | 'grants'
const collectionDefinition = {
  resourceKind: 'asset',
  statuses: [''],
  views: ['grid', 'list'],
  sortKeys: ['createdAt', 'filename', 'size'],
  pageSizes: [12, 24, 48, 96],
  defaultStatus: '',
  defaultView: 'grid',
  defaultSort: 'createdAt',
  defaultDirection: 'desc',
  defaultPageSize: 24,
  pagination: 'server',
  selection: 'page',
  filters: ['section', 'spaceKey', 'siteKey', 'profileKey', 'visibility', 'mime', 'task']
} as const satisfies ManageCollectionDefinition

const {
  searchInput,
  q,
  sort,
  direction,
  page,
  size,
  view: libraryView,
  state: collectionState,
  filterModel
} = useManageCollectionState({
  definition: collectionDefinition,
  routeQuery: computed(() => route.query),
  replaceQuery: query => router.replace({ query })
})
const section = filterModel('section', 'library')
const tab = computed<AssetAdminSection>({
  get: () => ['library', 'sites', 'profiles', 'storage', 'maintenance', 'grants'].includes(section.value)
    ? section.value as AssetAdminSection
    : 'library',
  set: value => { section.value = value }
})
const siteKey = filterModel('siteKey', ALL)
const spaceKey = filterModel('spaceKey', ALL)
const profileKey = filterModel('profileKey', ALL)
const visibility = filterModel('visibility', ALL)
const mime = filterModel('mime', ALL)
const selectedTaskId = filterModel('task', '')
const stats = ref<Stats>({ assets: 0, publicAssets: 0, privateAssets: 0, sites: 0, profiles: 0, activeGrants: 0 })
const sites = ref<Site[]>([])
const storageBackends = ref<StorageBackend[]>([])
const profiles = ref<Profile[]>([])
const variants = ref<Variant[]>([])
const assets = ref<AssetItem[]>([])
const spaces = ref<AssetSpaceUsage[]>([])
const references = ref<AssetReference[]>([])
const grants = ref<Grant[]>([])
const maintenanceTasks = ref<MaintenanceTask[]>([])
const controllingMaintenanceTaskId = ref('')
const queueingSelectedRebuild = ref(false)
const selectedRebuildError = ref('')
const totalAssets = ref(0)
const totalGrants = ref(0)
const grantPage = ref(1)
const GRANT_PAGE_SIZE = 20
const showSkeleton = useMinLoading(computed(() => !mounted.value || loading.value))
const enabledStorageBackends = computed(() => storageBackends.value.filter(b => b.enabled !== false))
const activeMaintenanceCount = computed(() => maintenanceTasks.value.filter(task =>
  task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
).length)
const sectionItems = computed<Array<{ label: string, icon: string, value: AssetAdminSection, count: number }>>(() => [
  { label: '素材库', icon: 'i-tabler-photo', value: 'library', count: stats.value.assets },
  { label: '站点', icon: 'i-tabler-world', value: 'sites', count: stats.value.sites },
  { label: 'Profile', icon: 'i-tabler-folder-cog', value: 'profiles', count: stats.value.profiles },
  { label: '存储', icon: 'i-tabler-database', value: 'storage', count: storageBackends.value.length },
  { label: '维护', icon: 'i-tabler-tool', value: 'maintenance', count: activeMaintenanceCount.value || maintenanceTasks.value.length },
  { label: '授权', icon: 'i-tabler-key', value: 'grants', count: stats.value.activeGrants }
])

const siteOptions = computed(() => [
  { label: '全部站点', value: ALL },
  ...sites.value.map(s => ({ label: `${s.name} / ${s.siteKey}`, value: s.siteKey }))
])
const profileOptions = computed(() => [
  { label: '全部 Profile', value: ALL },
  ...profiles.value
    .filter(p => siteKey.value === ALL || p.siteKey === siteKey.value)
    .map(p => ({ label: `${p.profileKey} / ${p.siteKey}`, value: p.profileKey }))
])
const visibilityOptions = [
  { label: '全部可见性', value: ALL },
  { label: '公开', value: 'public' },
  { label: '私有', value: 'private' }
]
const mimeOptions = [
  { label: '全部类型', value: ALL },
  { label: '图片', value: 'image/' },
  { label: '视频', value: 'video/' },
  { label: '音频', value: 'audio/' },
  { label: 'PDF', value: 'application/pdf' },
  { label: '其它文件', value: 'application/' }
]
const sortOptions = [
  { label: '按上传时间', value: 'createdAt' },
  { label: '按文件名', value: 'filename' },
  { label: '按文件大小', value: 'size' }
]
const pageSizeItems = [12, 24, 48, 96].map(value => ({ label: `${value}/页`, value }))
const totalAssetPages = computed(() => Math.max(1, Math.ceil(totalAssets.value / size.value)))
const activeLibraryFilterCount = computed(() => [spaceKey.value, siteKey.value, profileKey.value, visibility.value, mime.value]
  .filter(value => value !== ALL).length)
const storageBackendOptions = computed(() => storageBackends.value
  .filter(b => b.enabled !== false)
  .map(b => ({
    label: b.isDefault ? `${b.name} · 默认` : b.name,
    value: b.name
  })))
const profileStorageBackendOptions = computed(() => [
  { label: `继承站点默认 (${siteDefaultBackend(profileForm.siteKey)})`, value: '' },
  ...storageBackendOptions.value
])
const spaceOptions = computed(() => [
  { label: '全部资源空间', value: ALL },
  ...spaces.value.map(space => ({
    label: `${space.spaceKey} · ${space.assetCount} 个 / ${formatBytes(space.totalBytes)}`,
    value: space.spaceKey
  }))
])
const modeOptions = [
  { label: '等比缩放', value: 'resize' },
  { label: '填充裁剪', value: 'fill' }
]
const publicPolicyOptions = [{ label: '公开直链', value: 'public' }]
const privatePolicyOptions = [
  { label: '短期签名', value: 'signed' },
  { label: '一次性', value: 'oneTime' },
  { label: '付费', value: 'paid' },
  { label: '门禁', value: 'gated' }
]
const policyOptions = [...publicPolicyOptions, ...privatePolicyOptions]
const profileAccessLevelOptions = [
  { label: '公开资源 · 公开直链', value: 'public' },
  { label: '私有资源 · 签名链接', value: 'private' }
]
const hasActiveMaintenanceTask = computed(() => maintenanceTasks.value.some(task =>
  task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
))
let maintenanceTasksPollTimer: ReturnType<typeof setInterval> | undefined
let maintenanceTasksPollInFlight = false
let maintenanceTasksPollErrorShown = false

function toggleSortDirection() {
  direction.value = direction.value === 'desc' ? 'asc' : 'desc'
}

const selectionResetKey = computed(() => manageCollectionQueryFingerprint(
  serializeManageCollectionQuery(collectionState.value, collectionDefinition)
))
const {
  selectedIds,
  selectionCount,
  isPageSelected,
  isPageIndeterminate,
  isSelected,
  toggleOne,
  togglePage,
  clear: clearSelection
} = useManageSelection({
  visibleIds: computed(() => assets.value.map(asset => asset.id)),
  filteredTotal: computed(() => totalAssets.value),
  resetKey: selectionResetKey
})

const selectedTask = computed(() => maintenanceTasks.value.find(task => task.id === selectedTaskId.value))

function parseTaskResult(value?: string): MaintenanceTaskResult {
  if (!value || value === '{}') return {}
  try {
    return JSON.parse(value) as MaintenanceTaskResult
  } catch {
    return {}
  }
}

function taskResultSummary(task: MaintenanceTask) {
  const result = parseTaskResult(task.result)
  if (result.candidates == null) return ''
  const processed = (result.rebuilt || 0) + (result.errors?.length || 0)
  const parts = [`${processed}/${result.candidates} 已处理`]
  if (result.generated) parts.push(`生成 ${result.generated}`)
  if (result.errors?.length) parts.push(`${result.errors.length} 个未完成`)
  return parts.join(' · ')
}

function openMaintenanceTaskList() {
  tab.value = 'maintenance'
}

function dismissSelectedTask() {
  selectedTaskId.value = ''
  selectedRebuildError.value = ''
}

function controlSelectedTask(action: 'pause' | 'resume' | 'cancel') {
  if (selectedTask.value) void controlMaintenanceTask(selectedTask.value, action, true)
}

async function queueSelectedRebuild() {
  if (!selectedIds.value.length || queueingSelectedRebuild.value) return
  const ids = [...selectedIds.value]
  queueingSelectedRebuild.value = true
  selectedRebuildError.value = ''
  try {
    const response = await call<{ task?: MaintenanceTask }>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
      method: 'POST',
      body: { ids, dryRun: false }
    })
    if (!response.task?.id) throw new Error('后台未返回任务 ID')
    maintenanceTasks.value = [response.task, ...maintenanceTasks.value.filter(task => task.id !== response.task?.id)]
    selectedTaskId.value = response.task.id
    clearSelection()
  } catch (e) {
    selectedRebuildError.value = (e as Error)?.message || '无法创建后台任务，请稍后重试。'
  } finally {
    queueingSelectedRebuild.value = false
  }
}

onMounted(async () => {
  mounted.value = true
  await reloadAll()
})

watch([q, sort, direction, page, size, siteKey, spaceKey, profileKey, visibility, mime], fetchAssets)
watch(totalAssetPages, (lastPage) => {
  if (page.value > lastPage) page.value = lastPage
}, { flush: 'sync' })
watch(grantPage, fetchGrants)
watch(() => profileForm.defaultVisibility, (visibility) => {
  if (visibility === 'public') {
    profileForm.defaultDeliveryPolicy = 'public'
    return
  }
  if (!profileForm.defaultDeliveryPolicy || profileForm.defaultDeliveryPolicy === 'public') {
    profileForm.defaultDeliveryPolicy = 'signed'
  }
})
watch(hasActiveMaintenanceTask, (active) => {
  if (active && !maintenanceTasksPollTimer) {
    maintenanceTasksPollTimer = setInterval(() => { void pollMaintenanceTasks() }, 5000)
    return
  }
  if (!active && maintenanceTasksPollTimer) {
    clearInterval(maintenanceTasksPollTimer)
    maintenanceTasksPollTimer = undefined
  }
}, { immediate: true })

onBeforeUnmount(() => {
  if (maintenanceTasksPollTimer) {
    clearInterval(maintenanceTasksPollTimer)
    maintenanceTasksPollTimer = undefined
  }
})

async function reloadAll() {
  loading.value = true
  try {
    const [st, spaceData, siteData, backendData, profileData, variantData] = await Promise.all([
      call<Stats>('/api/v1/admin/assets-proxy/stats'),
      call<{ items: AssetSpaceUsage[] }>('/api/v1/admin/assets-proxy/spaces'),
      call<{ items: Site[] }>('/api/v1/admin/assets-proxy/sites'),
      call<{ items: StorageBackend[], defaultName: string }>('/api/v1/admin/assets-proxy/storage-backends'),
      call<{ items: Profile[] }>('/api/v1/admin/assets-proxy/profiles'),
      call<{ items: Variant[] }>('/api/v1/admin/assets-proxy/variants')
    ])
    stats.value = st
    spaces.value = spaceData.items ?? []
    sites.value = siteData.items ?? []
    storageBackends.value = backendData.items ?? []
    profiles.value = profileData.items ?? []
    variants.value = variantData.items ?? []
    await Promise.all([fetchAssets(), fetchGrants(), fetchMaintenanceTasks()])
  } catch (e) {
    toast.add({ title: '资源后台加载失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    loading.value = false
  }
}

async function fetchAssets() {
  const data = await call<{ items: AssetItem[], total: number }>('/api/v1/admin/assets-proxy/library', {
    params: {
      q: q.value || undefined,
      page: page.value,
      size: size.value,
      spaceKey: spaceKey.value !== ALL ? spaceKey.value : undefined,
      siteKey: siteKey.value !== ALL ? siteKey.value : undefined,
      profileKey: profileKey.value !== ALL ? profileKey.value : undefined,
      visibility: visibility.value !== ALL ? visibility.value : undefined,
      mime: mime.value !== ALL ? mime.value : undefined,
      sortBy: sort.value,
      sortOrder: direction.value
    }
  })
  assets.value = data.items ?? []
  totalAssets.value = data.total ?? 0
}

async function fetchGrants() {
  const data = await call<{ items: Grant[], total: number }>('/api/v1/admin/assets-proxy/grants', {
    params: { page: grantPage.value, size: GRANT_PAGE_SIZE }
  })
  grants.value = data.items ?? []
  totalGrants.value = data.total ?? 0
}

async function reloadStorageBackends() {
  const data = await call<{ items: StorageBackend[], defaultName: string }>('/api/v1/admin/assets-proxy/storage-backends')
  storageBackends.value = data.items ?? []
}

async function refreshAfterMaintenanceTasksSettled() {
  const [st, siteData, profileData, variantData] = await Promise.all([
    call<Stats>('/api/v1/admin/assets-proxy/stats'),
    call<{ items: Site[] }>('/api/v1/admin/assets-proxy/sites'),
    call<{ items: Profile[] }>('/api/v1/admin/assets-proxy/profiles'),
    call<{ items: Variant[] }>('/api/v1/admin/assets-proxy/variants')
  ])
  stats.value = st
  sites.value = siteData.items ?? []
  profiles.value = profileData.items ?? []
  variants.value = variantData.items ?? []
  await fetchAssets()
}

async function fetchMaintenanceTasks() {
  const data = await call<{ items: MaintenanceTask[] }>('/api/v1/admin/assets-proxy/maintenance/tasks', {
    params: { page: 1, size: 8 }
  })
  const items = data.items ?? []
  if (selectedTaskId.value && !items.some(task => task.id === selectedTaskId.value)) {
    try {
      const selected = await call<{ task: MaintenanceTask }>(`/api/v1/admin/assets-proxy/maintenance/tasks/${selectedTaskId.value}`)
      items.unshift(selected.task)
    } catch {
      // Keep the recent task list useful even when an old deep-linked task no longer exists.
    }
  }
  maintenanceTasks.value = items
}

async function pollMaintenanceTasks() {
  if (maintenanceTasksPollInFlight) return
  maintenanceTasksPollInFlight = true
  const hadActiveTask = hasActiveMaintenanceTask.value
  try {
    await fetchMaintenanceTasks()
    if (hadActiveTask && !hasActiveMaintenanceTask.value) {
      await refreshAfterMaintenanceTasksSettled()
    }
    maintenanceTasksPollErrorShown = false
  } catch (e) {
    if (!maintenanceTasksPollErrorShown) {
      toast.add({ title: '维护任务状态刷新失败', description: (e as Error)?.message, color: 'warning' })
      maintenanceTasksPollErrorShown = true
    }
  } finally {
    maintenanceTasksPollInFlight = false
  }
}

function showQueuedMaintenanceTask(title: string, task: MaintenanceTask) {
  toast.add({
    title,
    description: `任务 ${task.id.slice(0, 8)} 已进入队列`,
    color: 'success',
    icon: 'i-tabler-clock-check'
  })
}

function canPauseTask(task: MaintenanceTask) {
  return task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
}

function canResumeTask(task: MaintenanceTask) {
  return task.status === 'paused'
}

function canCancelTask(task: MaintenanceTask) {
  return task.status === 'queued' || task.status === 'running' || task.status === 'retrying' || task.status === 'paused'
}

async function controlMaintenanceTask(task: MaintenanceTask, action: 'pause' | 'resume' | 'cancel', silent = false) {
  controllingMaintenanceTaskId.value = `${task.id}:${action}`
  try {
    await call(`/api/v1/admin/assets-proxy/maintenance/tasks/${task.id}/${action}`, { method: 'POST' })
    if (!silent) {
      toast.add({
        title: action === 'pause' ? '维护任务已暂停' : action === 'resume' ? '维护任务已恢复' : '维护任务已取消',
        color: action === 'cancel' ? 'warning' : 'success',
        icon: action === 'pause' ? 'i-tabler-player-pause' : action === 'resume' ? 'i-tabler-player-play' : 'i-tabler-ban'
      })
    }
    await fetchMaintenanceTasks()
  } catch (e) {
    if (silent) selectedRebuildError.value = (e as Error)?.message || '维护任务操作失败'
    else toast.add({ title: '维护任务操作失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    controllingMaintenanceTaskId.value = ''
  }
}

const profileOpen = ref(false)
const profileForm = reactive<Profile>({
  siteKey: 'platform',
  profileKey: '',
  purpose: '',
  storageBackend: '',
  allowedExt: 'jpg,jpeg,png,webp',
  maxSizeBytes: 20 * 1024 * 1024,
  defaultVisibility: 'public',
  defaultDeliveryPolicy: 'public',
  keepOriginal: true,
  assetCount: 0,
  variantCount: 0
})
function editProfile(p?: Profile) {
  Object.assign(profileForm, p ?? {
    siteKey: sites.value[0]?.siteKey || 'platform',
    profileKey: '',
    purpose: '',
    storageBackend: '',
    allowedExt: 'jpg,jpeg,png,webp',
    maxSizeBytes: 20 * 1024 * 1024,
    defaultVisibility: 'public',
    defaultDeliveryPolicy: 'public',
    keepOriginal: true,
    assetCount: 0,
    variantCount: 0
  })
  profileOpen.value = true
}
async function saveProfile() {
  if (profileForm.defaultVisibility === 'public') {
    profileForm.defaultDeliveryPolicy = 'public'
  } else if (profileForm.defaultDeliveryPolicy !== 'signed') {
    profileForm.defaultDeliveryPolicy = 'signed'
  }
  await call('/api/v1/admin/assets-proxy/profiles', {
    method: 'POST',
    body: {
      ...profileForm,
      defaultDeliveryPolicy: profileForm.defaultVisibility === 'public' ? 'public' : profileForm.defaultDeliveryPolicy
    }
  })
  toast.add({ title: 'Profile 已保存', color: 'success', icon: 'i-tabler-check' })
  profileOpen.value = false
  await reloadAll()
}

function siteDefaultBackend(key: string) {
  return sites.value.find(s => s.siteKey === key)?.defaultStorageBackend
    || storageBackends.value.find(b => b.isDefault)?.name
    || storageBackends.value[0]?.name
    || 'local'
}

function profileStorageText(profile: Profile) {
  return profile.storageBackend || `继承 ${siteDefaultBackend(profile.siteKey)}`
}

function profileAccessText(profile: Profile) {
  return profile.defaultVisibility === 'private' ? '私有签名链接' : '公开直链'
}

const siteOpen = ref(false)
const siteForm = reactive<Site>({
  siteKey: '',
  name: '',
  defaultStorageBackend: 'local',
  enabled: true,
  assetCount: 0,
  profileCount: 0,
  variantCount: 0
})
function editSite(site?: Site) {
  Object.assign(siteForm, site ?? {
    siteKey: '',
    name: '',
    defaultStorageBackend: storageBackends.value.find(b => b.isDefault)?.name || storageBackends.value[0]?.name || 'local',
    enabled: true,
    assetCount: 0,
    profileCount: 0,
    variantCount: 0
  })
  siteOpen.value = true
}
async function saveSite() {
  await call('/api/v1/admin/assets-proxy/sites', { method: 'POST', body: siteForm })
  toast.add({ title: '站点已保存', color: 'success', icon: 'i-tabler-check' })
  siteOpen.value = false
  await reloadAll()
}

const storageBackendOpen = ref(false)
const savingStorageBackend = ref(false)
const loadingStorageBackend = ref(false)
const storageBackendEditingName = ref('')
const storageBackendDetail = ref<StorageBackendDetail | null>(null)
const storageBackendEvents = ref<StorageBackendEvent[]>([])
const checkingStorageBackend = ref(false)
const rotatingStorageBackend = ref(false)
const rotateStorageBackendOpen = ref(false)
const rotateStorageBackendSecret = ref('')
const deleteStorageBackendOpen = ref(false)
const deletingStorageBackend = ref(false)
const storageBackendForm = reactive({
  name: '',
  type: 's3',
  enabled: true,
  endpoint: '',
  region: 'us-east-1',
  bucketPublic: '',
  bucketPrivate: '',
  accessKey: '',
  secretKey: '',
  publicBaseUrl: '',
  pathStyle: true,
  useSsl: false
})
const storageBackendTypeItems = [
  { label: 'S3 兼容', value: 's3' },
  { label: '腾讯云 COS', value: 'cos' },
  { label: '阿里云 OSS', value: 'oss' }
]
function assignStorageBackendForm(detail?: Partial<StorageBackendDetail>) {
  Object.assign(storageBackendForm, {
    name: detail?.name || '',
    type: detail?.type || 's3',
    enabled: detail?.enabled ?? true,
    endpoint: detail?.endpoint || '',
    region: detail?.region || 'us-east-1',
    bucketPublic: detail?.bucketPublic || '',
    bucketPrivate: detail?.bucketPrivate || '',
    accessKey: detail?.accessKey || '',
    secretKey: '',
    publicBaseUrl: detail?.publicBaseUrl || '',
    pathStyle: detail?.pathStyle ?? true,
    useSsl: detail?.useSsl ?? false
  })
}
async function editStorageBackend(backend?: StorageBackend) {
  storageBackendEditingName.value = backend?.managed ? backend.name : ''
  storageBackendDetail.value = null
  storageBackendEvents.value = []
  rotateStorageBackendSecret.value = ''
  assignStorageBackendForm({
    name: backend?.name || '',
    type: backend?.type || 's3',
    enabled: backend?.enabled !== false,
  })
  storageBackendOpen.value = true
  if (!backend?.managed) return
  loadingStorageBackend.value = true
  try {
    const data = await call<{ backend: StorageBackendDetail }>(`/api/v1/admin/assets-proxy/storage-backends/${backend.name}`)
    assignStorageBackendForm(data.backend)
    storageBackendDetail.value = data.backend
    await fetchStorageBackendEvents(backend.name)
  } catch (e) {
    toast.add({ title: '加载存储后端失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    loadingStorageBackend.value = false
  }
}
async function saveStorageBackend() {
  savingStorageBackend.value = true
  try {
    await call('/api/v1/admin/assets-proxy/storage-backends', { method: 'POST', body: storageBackendForm })
    toast.add({ title: '存储后端已保存', color: 'success', icon: 'i-tabler-check' })
    storageBackendOpen.value = false
    await reloadAll()
  } catch (e) {
    toast.add({ title: '保存存储后端失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    savingStorageBackend.value = false
  }
}
async function confirmDeleteStorageBackend() {
  if (!storageBackendEditingName.value) return
  deletingStorageBackend.value = true
  try {
    await call(`/api/v1/admin/assets-proxy/storage-backends/${storageBackendEditingName.value}`, { method: 'DELETE' })
    toast.add({ title: '存储后端已删除', color: 'success', icon: 'i-tabler-check' })
    deleteStorageBackendOpen.value = false
    storageBackendOpen.value = false
    storageBackendEditingName.value = ''
    await reloadAll()
  } catch (e) {
    toast.add({ title: '删除存储后端失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    deletingStorageBackend.value = false
  }
}
async function fetchStorageBackendEvents(name = storageBackendEditingName.value) {
  if (!name) {
    storageBackendEvents.value = []
    return
  }
  const data = await call<{ items: StorageBackendEvent[] }>(`/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(name)}/events`)
  storageBackendEvents.value = data.items ?? []
}
async function checkStorageBackendHealth() {
  if (!storageBackendEditingName.value) return
  checkingStorageBackend.value = true
  try {
    const data = await call<{ backend: StorageBackendDetail }>(
      `/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(storageBackendEditingName.value)}/health-check`,
      { method: 'POST' }
    )
    storageBackendDetail.value = data.backend
    toast.add({
      title: data.backend.lastHealthOk === false ? '健康检查失败' : '健康检查通过',
      description: data.backend.lastHealthError || undefined,
      color: data.backend.lastHealthOk === false ? 'error' : 'success',
      icon: data.backend.lastHealthOk === false ? 'i-tabler-alert-triangle' : 'i-tabler-heartbeat'
    })
    await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
  } catch (e) {
    toast.add({ title: '健康检查失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    checkingStorageBackend.value = false
  }
}
async function confirmRotateStorageBackendSecret() {
  if (!storageBackendEditingName.value || !rotateStorageBackendSecret.value) return
  rotatingStorageBackend.value = true
  try {
    const data = await call<{ backend: StorageBackendDetail }>(
      `/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(storageBackendEditingName.value)}/rotate-secret`,
      { method: 'POST', body: { secretKey: rotateStorageBackendSecret.value } }
    )
    storageBackendDetail.value = data.backend
    rotateStorageBackendSecret.value = ''
    rotateStorageBackendOpen.value = false
    toast.add({ title: '密钥已轮换', color: 'success', icon: 'i-tabler-key' })
    await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
  } catch (e) {
    toast.add({ title: '密钥轮换失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    rotatingStorageBackend.value = false
  }
}

const variantOpen = ref(false)
const variantForm = reactive<Variant>({
  id: '',
  siteKey: 'platform',
  profileKey: 'default',
  variantKey: '',
  width: 800,
  height: 600,
  mode: 'resize',
  format: 'source',
  quality: 85,
  version: 1,
  enabled: true
})
function editVariant(v?: Variant, p?: Profile) {
  const fallbackSite = siteKey.value !== ALL ? siteKey.value : 'platform'
  Object.assign(variantForm, v ?? {
    id: '',
    siteKey: p?.siteKey || fallbackSite,
    profileKey: p?.profileKey || 'default',
    variantKey: '',
    width: 800,
    height: 600,
    mode: 'resize',
    format: 'source',
    quality: 85,
    version: 1,
    enabled: true
  })
  variantOpen.value = true
}
async function saveVariant() {
  await call('/api/v1/admin/assets-proxy/variants', { method: 'POST', body: variantForm })
  toast.add({ title: 'Variant 已保存', color: 'success', icon: 'i-tabler-check' })
  variantOpen.value = false
  await reloadAll()
}

const deleteAssetTarget = ref<AssetItem | null>(null)
const deleteAssetOpen = computed({ get: () => !!deleteAssetTarget.value, set: v => { if (!v) deleteAssetTarget.value = null } })
const deletingAsset = ref(false)
const rebuildingAssetId = ref('')
const sweepingStaging = ref(false)
const pruneOpen = ref(false)
const pruningUnreferenced = ref(false)
const prunePreview = ref<PruneResult | null>(null)
const pruneForm = reactive({ olderThanDays: 30, limit: 50 })
const orphanObjectsOpen = ref(false)
const auditingOrphanObjects = ref(false)
const orphanObjectsPreview = ref<OrphanObjectResult | null>(null)
const orphanObjectsForm = reactive({ olderThanDays: 7, limit: 100, backend: ALL })
const storageMigrationOpen = ref(false)
const migratingStorage = ref(false)
const storageMigrationPreview = ref<StorageMigrationResult | null>(null)
const storageMigrationForm = reactive({ sourceBackend: '', targetBackend: '', limit: 50 })
const batchRebuildOpen = ref(false)
const batchRebuilding = ref(false)
const batchRebuildProfile = ref<Profile | null>(null)
const batchRebuildPreview = ref<BatchRebuildResult | null>(null)
const batchRebuildLimit = ref(50)
async function confirmDeleteAsset() {
  if (!deleteAssetTarget.value) return
  deletingAsset.value = true
  try {
    await call(`/api/v1/admin/assets-proxy/library/${deleteAssetTarget.value.id}`, { method: 'DELETE' })
    toast.add({ title: '素材已删除', color: 'success', icon: 'i-tabler-check' })
    deleteAssetTarget.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '删除素材失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    deletingAsset.value = false
  }
}

async function rebuildDerivatives(asset: AssetItem) {
  rebuildingAssetId.value = asset.id
  try {
    const data = await call<{ generated: number }>(`/api/v1/admin/assets-proxy/library/${asset.id}/derivatives/rebuild`, { method: 'POST' })
    toast.add({ title: '派生图已重建', description: `生成 ${data.generated ?? 0} 个 Variant`, color: 'success', icon: 'i-tabler-check' })
  } catch (e) {
    toast.add({ title: '重建派生图失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    rebuildingAssetId.value = ''
  }
}

async function previewBatchRebuild(profile: Profile) {
  batchRebuildProfile.value = profile
  batchRebuildPreview.value = null
  batchRebuilding.value = true
  try {
    batchRebuildPreview.value = await call<BatchRebuildResult>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
      method: 'POST',
      body: { siteKey: profile.siteKey, profileKey: profile.profileKey, limit: batchRebuildLimit.value, dryRun: true }
    })
    await fetchMaintenanceTasks()
    batchRebuildOpen.value = true
  } catch (e) {
    toast.add({ title: '批量重建预检失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    batchRebuilding.value = false
  }
}

async function confirmBatchRebuild() {
  if (!batchRebuildProfile.value) return
  batchRebuilding.value = true
  try {
    const profile = batchRebuildProfile.value
    const data = await call<BatchRebuildResult>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
      method: 'POST',
      body: { siteKey: profile.siteKey, profileKey: profile.profileKey, limit: batchRebuildLimit.value, dryRun: false }
    })
    if (data.task) {
      showQueuedMaintenanceTask('批量派生图重建已排队', data.task)
      batchRebuildOpen.value = false
      batchRebuildPreview.value = null
      await fetchMaintenanceTasks()
      return
    }
    const failed = data.errors?.length ?? 0
    toast.add({
      title: '批量派生图重建完成',
      description: `处理 ${data.rebuilt ?? 0} 个素材，生成 ${data.generated ?? 0} 个 Variant${failed ? `，${failed} 个失败` : ''}`,
      color: failed ? 'warning' : 'success',
      icon: failed ? 'i-tabler-alert-triangle' : 'i-tabler-check'
    })
    batchRebuildOpen.value = false
    batchRebuildPreview.value = null
    await fetchMaintenanceTasks()
  } catch (e) {
    toast.add({ title: '批量重建失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    batchRebuilding.value = false
  }
}

async function sweepStaging() {
  sweepingStaging.value = true
  try {
    const data = await call<{ items: SweepResult[], removed: number }>('/api/v1/admin/assets-proxy/maintenance/sweep-staging', { method: 'POST' })
    const skipped = (data.items ?? []).filter(i => i.skipped).length
    const failed = (data.items ?? []).filter(i => i.error).length
    toast.add({
      title: '暂存清理完成',
      description: `删除 ${data.removed ?? 0} 个对象${skipped ? `，跳过 ${skipped} 个后端` : ''}${failed ? `，${failed} 个异常` : ''}`,
      color: failed ? 'warning' : 'success',
      icon: failed ? 'i-tabler-alert-triangle' : 'i-tabler-check'
    })
    await reloadAll()
  } catch (e) {
    toast.add({ title: '暂存清理失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    sweepingStaging.value = false
  }
}

async function previewPruneUnreferenced() {
  pruningUnreferenced.value = true
  try {
    prunePreview.value = await call<PruneResult>('/api/v1/admin/assets-proxy/maintenance/prune-unreferenced', {
      method: 'POST',
      body: { ...pruneForm, dryRun: true }
    })
    await fetchMaintenanceTasks()
    pruneOpen.value = true
  } catch (e) {
    toast.add({ title: '无引用素材预检失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    pruningUnreferenced.value = false
  }
}

async function confirmPruneUnreferenced() {
  pruningUnreferenced.value = true
  try {
    const data = await call<PruneResult>('/api/v1/admin/assets-proxy/maintenance/prune-unreferenced', {
      method: 'POST',
      body: { ...pruneForm, dryRun: false }
    })
    if (data.task) {
      showQueuedMaintenanceTask('无引用素材清理已排队', data.task)
      pruneOpen.value = false
      prunePreview.value = null
      await fetchMaintenanceTasks()
      return
    }
    const failed = data.errors?.length ?? 0
    toast.add({
      title: '无引用素材清理完成',
      description: `删除 ${data.deleted ?? 0} 个素材${failed ? `，${failed} 个失败` : ''}`,
      color: failed ? 'warning' : 'success',
      icon: failed ? 'i-tabler-alert-triangle' : 'i-tabler-check'
    })
    pruneOpen.value = false
    prunePreview.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '无引用素材清理失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    pruningUnreferenced.value = false
  }
}

async function previewOrphanObjects() {
  auditingOrphanObjects.value = true
  try {
    orphanObjectsPreview.value = await call<OrphanObjectResult>('/api/v1/admin/assets-proxy/maintenance/orphan-objects', {
      method: 'POST',
      body: {
        olderThanDays: orphanObjectsForm.olderThanDays,
        limit: orphanObjectsForm.limit,
        backend: orphanObjectsForm.backend !== ALL ? orphanObjectsForm.backend : '',
        dryRun: true
      }
    })
    await fetchMaintenanceTasks()
    orphanObjectsOpen.value = true
  } catch (e) {
    toast.add({ title: '孤儿对象预检失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    auditingOrphanObjects.value = false
  }
}

async function confirmPruneOrphanObjects() {
  auditingOrphanObjects.value = true
  try {
    const data = await call<OrphanObjectResult>('/api/v1/admin/assets-proxy/maintenance/orphan-objects', {
      method: 'POST',
      body: {
        olderThanDays: orphanObjectsForm.olderThanDays,
        limit: orphanObjectsForm.limit,
        backend: orphanObjectsForm.backend !== ALL ? orphanObjectsForm.backend : '',
        dryRun: false
      }
    })
    if (data.task) {
      showQueuedMaintenanceTask('孤儿对象清理已排队', data.task)
      orphanObjectsOpen.value = false
      orphanObjectsPreview.value = null
      await fetchMaintenanceTasks()
      return
    }
    const failed = (data.items ?? []).reduce((sum, item) => sum + (item.errors?.length ?? 0), 0)
    toast.add({
      title: '孤儿对象清理完成',
      description: `删除 ${data.deleted ?? 0} 个对象${failed ? `，${failed} 个失败` : ''}`,
      color: failed ? 'warning' : 'success',
      icon: failed ? 'i-tabler-alert-triangle' : 'i-tabler-check'
    })
    orphanObjectsOpen.value = false
    orphanObjectsPreview.value = null
    await fetchMaintenanceTasks()
  } catch (e) {
    toast.add({ title: '孤儿对象清理失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    auditingOrphanObjects.value = false
  }
}

function openStorageMigration() {
  const enabled = storageBackends.value.filter(b => b.enabled !== false)
  storageMigrationForm.sourceBackend = enabled[0]?.name || ''
  storageMigrationForm.targetBackend = enabled.find(b => b.name !== storageMigrationForm.sourceBackend)?.name || ''
  storageMigrationForm.limit = 50
  storageMigrationPreview.value = null
  storageMigrationOpen.value = true
}

async function previewStorageMigration() {
  migratingStorage.value = true
  try {
    storageMigrationPreview.value = await call<StorageMigrationResult>('/api/v1/admin/assets-proxy/maintenance/migrate-storage', {
      method: 'POST',
      body: { ...storageMigrationForm, dryRun: true }
    })
    await fetchMaintenanceTasks()
  } catch (e) {
    toast.add({ title: '存储迁移预检失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    migratingStorage.value = false
  }
}

async function confirmStorageMigration() {
  migratingStorage.value = true
  try {
    const data = await call<StorageMigrationResult>('/api/v1/admin/assets-proxy/maintenance/migrate-storage', {
      method: 'POST',
      body: { ...storageMigrationForm, dryRun: false }
    })
    if (data.task) {
      showQueuedMaintenanceTask('存储迁移已排队', data.task)
      storageMigrationOpen.value = false
      storageMigrationPreview.value = null
      await fetchMaintenanceTasks()
      return
    }
    const failed = data.errors?.length ?? 0
    toast.add({
      title: '存储迁移完成',
      description: `迁移 ${data.migrated ?? 0} 个素材${failed ? `，${failed} 个失败` : ''}`,
      color: failed ? 'warning' : 'success',
      icon: failed ? 'i-tabler-alert-triangle' : 'i-tabler-check'
    })
    storageMigrationOpen.value = false
    storageMigrationPreview.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '存储迁移失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    migratingStorage.value = false
  }
}

const referenceAsset = ref<AssetItem | null>(null)
const referencesOpen = computed({ get: () => !!referenceAsset.value, set: v => { if (!v) referenceAsset.value = null } })
const loadingReferences = ref(false)
async function openReferences(asset: AssetItem) {
  referenceAsset.value = asset
  references.value = []
  loadingReferences.value = true
  try {
    const data = await call<{ items: AssetReference[] }>('/api/v1/admin/assets-proxy/references', {
      params: { assetId: asset.id, page: 1, size: 50 }
    })
    references.value = data.items ?? []
  } catch (e) {
    toast.add({ title: '加载引用失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    loadingReferences.value = false
  }
}

const deleteVariantTarget = ref<Variant | null>(null)
const deleteVariantOpen = computed({ get: () => !!deleteVariantTarget.value, set: v => { if (!v) deleteVariantTarget.value = null } })
const deletingVariant = ref(false)
async function confirmDeleteVariant() {
  if (!deleteVariantTarget.value) return
  deletingVariant.value = true
  try {
    await call(`/api/v1/admin/assets-proxy/variants/${deleteVariantTarget.value.id}`, { method: 'DELETE' })
    toast.add({ title: 'Variant 已删除', color: 'success', icon: 'i-tabler-check' })
    deleteVariantTarget.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '删除 Variant 失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    deletingVariant.value = false
  }
}

const deleteProfileTarget = ref<Profile | null>(null)
const deleteProfileOpen = computed({ get: () => !!deleteProfileTarget.value, set: v => { if (!v) deleteProfileTarget.value = null } })
const deletingProfile = ref(false)
async function confirmDeleteProfile() {
  if (!deleteProfileTarget.value) return
  deletingProfile.value = true
  try {
    const p = deleteProfileTarget.value
    await call(`/api/v1/admin/assets-proxy/profiles/${p.siteKey}/${p.profileKey}`, { method: 'DELETE' })
    toast.add({ title: 'Profile 已删除', color: 'success', icon: 'i-tabler-check' })
    deleteProfileTarget.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '删除 Profile 失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    deletingProfile.value = false
  }
}

const revokeGrantTarget = ref<Grant | null>(null)
const revokeGrantOpen = computed({ get: () => !!revokeGrantTarget.value, set: v => { if (!v) revokeGrantTarget.value = null } })
const revokingGrant = ref(false)
const createGrantOpen = ref(false)
const creatingGrant = ref(false)
const grantAsset = ref<AssetItem | null>(null)
const createdGrant = ref<CreatedGrant | null>(null)
const grantForm = reactive({
  variantKey: 'original',
  policy: 'oneTime',
  subjectId: '',
  expiresIn: 3600,
  maxUses: 1,
  reason: 'admin-issued'
})
async function confirmRevokeGrant() {
  if (!revokeGrantTarget.value) return
  revokingGrant.value = true
  try {
    await call(`/api/v1/admin/assets-proxy/grants/${revokeGrantTarget.value.id}/revoke`, { method: 'POST' })
    toast.add({ title: '授权已撤销', color: 'success', icon: 'i-tabler-check' })
    revokeGrantTarget.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '撤销授权失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    revokingGrant.value = false
  }
}

function openCreateGrant(asset: AssetItem) {
  grantAsset.value = asset
  createdGrant.value = null
  Object.assign(grantForm, {
    variantKey: 'original',
    policy: asset.deliveryPolicy && asset.deliveryPolicy !== 'public' ? asset.deliveryPolicy : 'oneTime',
    subjectId: '',
    expiresIn: 3600,
    maxUses: 1,
    reason: `admin:${asset.filename || asset.id}`
  })
  createGrantOpen.value = true
}

async function createGrant() {
  if (!grantAsset.value) return
  creatingGrant.value = true
  try {
    createdGrant.value = await call<CreatedGrant>('/api/v1/admin/assets-proxy/grants/create', {
      method: 'POST',
      body: { ...grantForm, assetId: grantAsset.value.id }
    })
    toast.add({ title: '交付链接已生成', color: 'success', icon: 'i-tabler-check' })
    await fetchGrants()
  } catch (e) {
    toast.add({ title: '生成交付链接失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    creatingGrant.value = false
  }
}

async function copyCreatedGrant() {
  if (!createdGrant.value?.url) return
  await navigator.clipboard.writeText(createdGrant.value.url)
  toast.add({ title: '链接已复制', color: 'success', icon: 'i-tabler-copy-check' })
}

function formatBytes(n: number) {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`
}
function briefDate(s: string) {
  return s ? s.replace('T', ' ').slice(0, 16) : '-'
}
function variantsFor(p: Profile) {
  return variants.value.filter(v => v.siteKey === p.siteKey && v.profileKey === p.profileKey)
}
function siteName(key: string) {
  return sites.value.find(s => s.siteKey === key)?.name || key
}
function siteActions(site: Site): DropdownMenuItem[][] {
  return [[
    { label: '编辑站点', icon: 'i-tabler-pencil', onSelect: () => editSite(site) }
  ]]
}
function profileActions(profile: Profile): DropdownMenuItem[][] {
  const inUse = profile.assetCount > 0 || profile.variantCount > 0
  return [[
    { label: '编辑', icon: 'i-tabler-pencil', onSelect: () => editProfile(profile) },
    { label: '批量重建派生图', icon: 'i-tabler-refresh-dot', disabled: !profile.assetCount || batchRebuilding.value, onSelect: () => previewBatchRebuild(profile) },
    { label: '删除', icon: 'i-tabler-trash', color: 'error', disabled: inUse, onSelect: () => { deleteProfileTarget.value = profile } }
  ]]
}
function assetActions(asset: AssetItem): DropdownMenuItem[][] {
  return [[
    { label: '查看引用', icon: 'i-tabler-link', disabled: !asset.refCount, onSelect: () => openReferences(asset) },
    { label: '打开文件', icon: 'i-tabler-external-link', disabled: !asset.cdnUrl, onSelect: () => { if (asset.cdnUrl) window.open(asset.cdnUrl, '_blank') } },
    { label: '签发交付链接', icon: 'i-tabler-key', onSelect: () => openCreateGrant(asset) },
    { label: '重建派生图', icon: 'i-tabler-refresh-dot', disabled: !asset.mime.startsWith('image/') || rebuildingAssetId.value === asset.id, onSelect: () => rebuildDerivatives(asset) }
  ], [
    { label: asset.refCount ? '有引用，不能删除' : '删除素材', icon: 'i-tabler-trash', color: 'error', disabled: !!asset.refCount, onSelect: () => { deleteAssetTarget.value = asset } }
  ]]
}
function variantActions(variant: Variant): DropdownMenuItem[][] {
  return [[
    { label: '编辑', icon: 'i-tabler-pencil', onSelect: () => editVariant(variant) },
    { label: '删除', icon: 'i-tabler-trash', color: 'error', onSelect: () => { deleteVariantTarget.value = variant } }
  ]]
}
function grantStatus(grant: Grant): { label: string, color: 'success' | 'warning' | 'error' | 'neutral' } {
  if (grant.revokedAt) return { label: '已撤销', color: 'neutral' }
  if (grant.expiresAt && new Date(grant.expiresAt).getTime() <= Date.now()) return { label: '已过期', color: 'warning' }
  if (grant.maxUses > 0 && grant.usedCount >= grant.maxUses) return { label: '已用完', color: 'warning' }
  return { label: '有效', color: 'success' }
}
function grantPurposeLabel(grant: Grant) {
  return grant.purpose || 'delivery'
}
function grantLastUsedText(grant: Grant) {
  return grant.lastUsedAt ? `最近 ${briefDate(grant.lastUsedAt)}` : '尚未使用'
}
function grantAuditText(grant: Grant) {
  if (grant.revokedBy) return `撤销 ${grant.revokedBy}`
  return grant.createdByService ? `签发 ${grant.createdByService}` : '系统签发'
}
function canRevokeGrant(grant: Grant) {
  return grantStatus(grant).label === '有效'
}
function taskTypeLabel(type: string) {
  const labels: Record<string, string> = {
    'sweep-staging': '清理暂存',
    'prune-unreferenced': '无引用素材',
    'orphan-objects': '孤儿对象',
    'rebuild-derivatives': '重建派生图',
    'migrate-storage': '存储迁移'
  }
  return labels[type] || type
}
function taskStatus(task: MaintenanceTask): { label: string, color: 'success' | 'warning' | 'error' | 'neutral' } {
  if (task.status === 'failed') return { label: '失败', color: 'error' }
  if (task.dryRun) return { label: '预检', color: 'warning' }
  if (task.status === 'queued') return { label: '排队中', color: 'neutral' }
  if (task.status === 'running') return { label: '执行中', color: 'warning' }
  if (task.status === 'retrying') return { label: '重试中', color: 'warning' }
  if (task.status === 'paused') return { label: '已暂停', color: 'neutral' }
  if (task.status === 'completed') return { label: '完成', color: 'success' }
  if (task.status === 'cancelled') return { label: '已取消', color: 'neutral' }
  return { label: '未知', color: 'neutral' }
}
function storageBackendEventTypeLabel(type: string) {
  const labels: Record<string, string> = {
    config_upserted: '配置保存',
    config_deleted: '配置删除',
    health_check: '健康检查',
    secret_rotated: '密钥轮换'
  }
  return labels[type] || type
}
function storageBackendEventStatusColor(status: string): 'success' | 'warning' | 'error' | 'neutral' {
  if (status === 'ok') return 'success'
  if (status === 'error') return 'error'
  if (status === 'warning') return 'warning'
  return 'neutral'
}
function storageBackendHealthBadge(detail?: StorageBackendDetail | null): { label: string, color: 'success' | 'warning' | 'error' | 'neutral' } {
  if (detail?.lastHealthOk === true) return { label: '正常', color: 'success' }
  if (detail?.lastHealthOk === false) return { label: '异常', color: 'error' }
  return { label: '未检查', color: 'neutral' }
}
function openExternal(url: string) {
  if (url) window.open(url, '_blank')
}
function grantActions(grant: Grant): DropdownMenuItem[][] {
  return [[
    { label: '撤销授权', icon: 'i-tabler-ban', color: 'error', disabled: !canRevokeGrant(grant), onSelect: () => { revokeGrantTarget.value = grant } }
  ]]
}
</script>

<template>
  <div>
    <ManageHeader title="资源管理">
      <template #subtitle>站群共享素材控制面:站点、Profile、Variant、素材库和交付授权</template>
      <template #actions>
        <div class="flex items-center gap-2">
          <UButton icon="i-tabler-refresh" label="刷新" color="neutral" variant="soft" @click="reloadAll" />
          <UButton v-if="tab === 'sites'" icon="i-tabler-world-plus" label="新建站点" @click="editSite()" />
          <UButton v-else-if="tab === 'profiles'" icon="i-tabler-plus" label="新建 Profile" @click="editProfile()" />
          <UButton v-else-if="tab === 'storage'" icon="i-tabler-database-plus" label="新建后端" @click="editStorageBackend()" />
          <UButton
            v-else-if="tab === 'maintenance'"
            icon="i-tabler-transfer"
            label="迁移后端"
            :disabled="enabledStorageBackends.length < 2"
            @click="openStorageMigration"
          />
        </div>
      </template>
    </ManageHeader>

    <div class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
      <div
        v-for="item in [
          ['资源', stats.assets],
          ['公开', stats.publicAssets],
          ['私有', stats.privateAssets],
          ['站点', stats.sites],
          ['Profile', stats.profiles],
          ['有效授权', stats.activeGrants]
        ]"
        :key="item[0]"
        class="rounded-lg border border-default bg-default px-4 py-3"
      >
        <div class="text-xs text-muted">{{ item[0] }}</div>
        <div class="mt-1 text-xl font-semibold text-highlighted tabular-nums">{{ item[1] }}</div>
      </div>
    </div>

    <div class="mb-5 flex flex-wrap items-center gap-2">
      <UButton
        v-for="item in sectionItems"
        :key="item.value"
        :label="`${item.label} · ${item.count}`"
        :icon="item.icon"
        :color="tab === item.value ? 'primary' : 'neutral'"
        :variant="tab === item.value ? 'solid' : 'ghost'"
        @click="() => { tab = item.value }"
      />
    </div>

    <SkeletonList v-if="showSkeleton" :rows="8" />

    <template v-else>
      <section v-if="tab === 'library'" class="space-y-4">
        <ManageCollectionToolbar
          v-model:search="searchInput"
          search-placeholder="搜索文件名、标题、替代文本或 ID…"
          :filter-count="activeLibraryFilterCount"
        >
          <template #filters>
            <USelectMenu v-model="spaceKey" :items="spaceOptions" value-key="value" :search-input="{ placeholder: '搜索资源空间…' }" />
            <USelectMenu v-model="siteKey" :items="siteOptions" value-key="value" :search-input="{ placeholder: '搜索站点…' }" />
            <USelectMenu v-model="profileKey" :items="profileOptions" value-key="value" :search-input="{ placeholder: '搜索 Profile…' }" />
            <USelect v-model="visibility" :items="visibilityOptions" value-key="value" />
            <USelect v-model="mime" :items="mimeOptions" value-key="value" />
            <USelect v-model="sort" :items="sortOptions" value-key="value" icon="i-tabler-arrows-sort" />
            <UButton
              :icon="direction === 'desc' ? 'i-tabler-sort-descending' : 'i-tabler-sort-ascending'"
              :label="direction === 'desc' ? '降序' : '升序'"
              color="neutral"
              variant="outline"
              @click="toggleSortDirection"
            />
          </template>
          <template #actions>
            <ManageViewToggle v-model="libraryView" :items="[
              { key: 'grid', label: '网格', icon: 'i-tabler-layout-grid' },
              { key: 'list', label: '列表', icon: 'i-tabler-list' }
            ]" />
          </template>
        </ManageCollectionToolbar>

        <ManageEmpty v-if="!assets.length" icon="i-tabler-photo-off" text="没有匹配的资源" />
        <div v-else-if="libraryView === 'grid'" class="grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(min(14rem,100%),1fr))]">
          <article
            v-for="asset in assets"
            :key="asset.id"
            class="group relative overflow-hidden rounded-xl border border-default bg-default transition hover:-translate-y-0.5 hover:shadow-sm"
            :class="isSelected(asset.id) ? 'ring-2 ring-primary' : ''"
          >
            <div class="relative aspect-[4/3] overflow-hidden border-b border-default bg-elevated">
              <img v-if="asset.cdnUrl && asset.mime.startsWith('image/')" :src="asset.cdnUrl" :alt="asset.filename" class="size-full object-cover transition duration-300 group-hover:scale-[1.03]">
              <div v-else class="grid size-full place-items-center">
                <UIcon name="i-tabler-file" class="size-9 text-muted" />
              </div>
              <UCheckbox
                class="absolute left-2 top-2 rounded-md bg-default/90 p-1 backdrop-blur"
                :model-value="isSelected(asset.id)"
                :aria-label="`选择素材：${asset.filename || asset.id}`"
                @update:model-value="toggleOne(asset.id)"
              />
              <UDropdownMenu :items="assetActions(asset)">
                <UButton icon="i-tabler-dots-vertical" color="neutral" variant="solid" square size="xs" class="absolute right-2 top-2" :aria-label="`素材操作：${asset.filename || asset.id}`" />
              </UDropdownMenu>
            </div>
            <div class="min-w-0 p-3">
              <h2 class="truncate text-sm font-semibold text-highlighted">{{ asset.filename || asset.id }}</h2>
              <p class="mt-1 truncate text-xs text-muted">{{ siteName(asset.siteKey) }} · {{ asset.profileKey || 'default' }}</p>
              <div class="mt-3 flex items-end justify-between gap-3 border-t border-default pt-2.5 text-xs text-muted">
                <div class="min-w-0">
                  <p class="truncate">{{ formatBytes(asset.size) }}<span v-if="asset.width && asset.height"> · {{ asset.width }}×{{ asset.height }}</span></p>
                  <p class="mt-0.5 truncate">{{ asset.storageBackend || 'local' }}</p>
                </div>
                <span v-if="asset.refCount" class="shrink-0 text-warning">{{ asset.refCount }} 引用</span>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
          <ManageRowShell
            v-for="asset in assets"
            :key="asset.id"
            :selected="isSelected(asset.id)"
            :selection-label="`选择素材：${asset.filename || asset.id}`"
            @select="toggleOne(asset.id)"
          >
            <template #media>
              <div class="grid size-14 shrink-0 place-items-center overflow-hidden rounded-lg bg-elevated">
                <img v-if="asset.cdnUrl && asset.mime.startsWith('image/')" :src="asset.cdnUrl" :alt="asset.filename" class="size-full object-cover">
                <UIcon v-else name="i-tabler-file" class="size-5 text-muted" />
              </div>
            </template>
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-highlighted">{{ asset.filename || asset.id }}</p>
              <p class="mt-0.5 truncate text-xs text-muted">{{ asset.mime }} · {{ formatBytes(asset.size) }}<span v-if="asset.width && asset.height"> · {{ asset.width }}×{{ asset.height }}</span></p>
              <p class="mt-1 truncate text-xs text-dimmed">空间 {{ asset.spaceKey || 'default' }} · {{ siteName(asset.siteKey) }} / {{ asset.profileKey }}</p>
            </div>
            <template #meta>
              <div class="min-w-0 text-xs md:w-44 md:text-right">
                <p class="truncate text-default">{{ asset.storageBackend || 'local' }}</p>
                <p class="mt-0.5 truncate text-muted">{{ asset.visibility }} · {{ asset.deliveryPolicy || 'public' }}</p>
                <p v-if="asset.refCount" class="mt-1 text-warning">{{ asset.refCount }} 个引用</p>
              </div>
            </template>
            <template #actions>
              <UDropdownMenu :items="assetActions(asset)">
                <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="sm" :aria-label="`素材操作：${asset.filename || asset.id}`" />
              </UDropdownMenu>
            </template>
          </ManageRowShell>
        </div>

        <AssetMaintenanceDock
          v-if="totalAssets > 0 || assets.length"
          v-model:page="page"
          v-model:page-size="size"
          :total-pages="totalAssetPages"
          :page-size-items="pageSizeItems"
          :total="totalAssets"
          :selected-count="selectionCount"
          :page-selected="isPageSelected"
          :page-indeterminate="isPageIndeterminate"
          :queueing="queueingSelectedRebuild"
          :queue-error="selectedRebuildError"
          :task-id="selectedTaskId"
          :task="selectedTask"
          :controlling-task-id="controllingMaintenanceTaskId"
          @toggle-page="togglePage"
          @queue-selected="queueSelectedRebuild"
          @clear-selection="clearSelection"
          @task-action="controlSelectedTask"
          @open-maintenance="openMaintenanceTaskList"
          @dismiss-task="dismissSelectedTask"
        />
      </section>

      <section v-else-if="tab === 'storage'" class="space-y-4">
        <div class="rounded-lg border border-default bg-default p-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-highlighted">存储后端</h2>
              <p class="mt-1 text-xs text-muted">站点上传会落到自己的默认后端；不存在的后端不能保存。</p>
            </div>
            <div class="flex items-center gap-2">
              <UBadge :label="`${storageBackends.length} 个可用`" color="neutral" variant="soft" />
            </div>
          </div>
          <ManageEmpty v-if="!storageBackends.length" icon="i-tabler-database-off" text="还没有存储后端" />
          <div v-else class="mt-3 flex flex-wrap gap-2">
            <UButton
              v-for="backend in storageBackends"
              :key="backend.name"
              :label="`${backend.name}${backend.type ? ` · ${backend.type}` : ''}${backend.isDefault ? ' · 默认' : ''}${backend.enabled === false ? ' · 停用' : ''} · ${backend.assetCount || 0} 素材 / ${backend.siteCount || 0} 站点 / ${backend.profileCount || 0} Profile${backend.healthy ? '' : ' · 异常'}`"
              :color="backend.enabled === false ? 'neutral' : (backend.healthy ? (backend.isDefault ? 'primary' : 'neutral') : 'error')"
              :icon="backend.managed ? 'i-tabler-pencil' : 'i-tabler-database'"
              variant="soft"
              size="xs"
              :disabled="!backend.managed"
              @click="editStorageBackend(backend)"
            />
          </div>
        </div>
      </section>

      <section v-else-if="tab === 'maintenance'" class="space-y-4">
        <div class="rounded-lg border border-default bg-default p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-highlighted">维护任务</h2>
              <p class="mt-1 text-xs text-muted">低频清理、扫描和迁移操作集中在这里执行。</p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <UButton
                icon="i-tabler-broom"
                label="清理暂存"
                color="neutral"
                variant="soft"
                size="xs"
                :loading="sweepingStaging"
                @click="sweepStaging"
              />
              <UButton
                icon="i-tabler-unlink"
                label="清理无引用"
                color="neutral"
                variant="soft"
                size="xs"
                :loading="pruningUnreferenced"
                @click="previewPruneUnreferenced"
              />
              <UButton
                icon="i-tabler-database-search"
                label="扫描孤儿对象"
                color="neutral"
                variant="soft"
                size="xs"
                :loading="auditingOrphanObjects"
                @click="previewOrphanObjects"
              />
            </div>
          </div>
          <div class="mt-4 rounded-lg border border-default bg-elevated/30">
            <div class="flex items-center justify-between border-b border-default px-3 py-2">
              <h3 class="text-xs font-medium text-muted">最近维护</h3>
              <UButton icon="i-tabler-refresh" color="neutral" variant="ghost" square size="xs" @click="fetchMaintenanceTasks" />
            </div>
            <ManageEmpty v-if="!maintenanceTasks.length" icon="i-tabler-history" text="还没有维护记录" />
            <div v-else class="divide-y divide-default">
              <div v-for="task in maintenanceTasks" :key="task.id" class="flex items-center gap-3 px-3 py-2.5">
                <span class="grid size-8 shrink-0 place-items-center rounded-lg bg-default text-muted">
                  <UIcon name="i-tabler-history" class="size-4" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex min-w-0 items-center gap-2">
                    <span class="truncate text-sm font-medium text-highlighted">{{ task.summary || taskTypeLabel(task.taskType) }}</span>
                    <UBadge :label="taskStatus(task).label" :color="taskStatus(task).color" variant="soft" size="sm" />
                  </div>
                  <div class="mt-0.5 truncate text-xs text-muted">{{ taskTypeLabel(task.taskType) }} · {{ briefDate(task.createdAt) }}</div>
                  <div v-if="taskResultSummary(task)" class="mt-0.5 truncate text-xs text-muted">{{ taskResultSummary(task) }}</div>
                </div>
                <div v-if="task.error" class="hidden max-w-64 truncate text-xs text-error md:block">{{ task.error }}</div>
                <div class="flex shrink-0 items-center gap-1">
                  <UButton
                    v-if="canPauseTask(task)"
                    icon="i-tabler-player-pause"
                    color="neutral"
                    variant="ghost"
                    square
                    size="xs"
                    :loading="controllingMaintenanceTaskId === `${task.id}:pause`"
                    @click.stop="controlMaintenanceTask(task, 'pause')"
                  />
                  <UButton
                    v-if="canResumeTask(task)"
                    icon="i-tabler-player-play"
                    color="neutral"
                    variant="ghost"
                    square
                    size="xs"
                    :loading="controllingMaintenanceTaskId === `${task.id}:resume`"
                    @click.stop="controlMaintenanceTask(task, 'resume')"
                  />
                  <UButton
                    v-if="canCancelTask(task)"
                    icon="i-tabler-ban"
                    color="error"
                    variant="ghost"
                    square
                    size="xs"
                    :loading="controllingMaintenanceTaskId === `${task.id}:cancel`"
                    @click.stop="controlMaintenanceTask(task, 'cancel')"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

      </section>

      <section v-else-if="tab === 'sites'" class="space-y-4">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">站点</h2>
          <p class="mt-1 text-xs text-muted">每个站点拥有自己的默认存储后端和 Profile/Variant 计数。</p>
        </div>
        <ManageEmpty v-if="!sites.length" icon="i-tabler-world-off" text="还没有站点配置" />
        <div v-else class="grid gap-3 lg:grid-cols-2">
          <div v-for="site in sites" :key="site.siteKey" class="rounded-lg border border-default bg-default p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                    <UIcon name="i-tabler-world" class="size-4" />
                  </span>
                  <div class="min-w-0">
                    <h3 class="truncate text-sm font-semibold text-highlighted">{{ site.name }}</h3>
                    <p class="truncate font-mono text-xs text-muted">{{ site.siteKey }}</p>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <UBadge :label="site.enabled ? '启用' : '停用'" :color="site.enabled ? 'success' : 'neutral'" variant="soft" />
                <UDropdownMenu :items="siteActions(site)">
                  <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="xs" />
                </UDropdownMenu>
              </div>
            </div>
            <div class="mt-4 grid grid-cols-3 gap-2 text-xs">
              <div class="rounded-md bg-elevated/40 px-3 py-2">
                <div class="text-muted">素材</div>
                <div class="mt-0.5 font-semibold text-highlighted tabular-nums">{{ site.assetCount }}</div>
              </div>
              <div class="rounded-md bg-elevated/40 px-3 py-2">
                <div class="text-muted">Profile</div>
                <div class="mt-0.5 font-semibold text-highlighted tabular-nums">{{ site.profileCount }}</div>
              </div>
              <div class="rounded-md bg-elevated/40 px-3 py-2">
                <div class="text-muted">Variant</div>
                <div class="mt-0.5 font-semibold text-highlighted tabular-nums">{{ site.variantCount }}</div>
              </div>
            </div>
            <div class="mt-3 text-xs text-muted">默认后端 <span class="font-mono text-default">{{ site.defaultStorageBackend }}</span></div>
          </div>
        </div>

      </section>

      <section v-else-if="tab === 'profiles'" class="space-y-4">
        <div>
          <h2 class="text-sm font-semibold text-highlighted">Profile 与派生规格</h2>
          <p class="mt-1 text-xs text-muted">按站点管理上传用途、文件限制、访问级别和命名派生规格。</p>
        </div>

        <ManageEmpty v-if="!profiles.length" icon="i-tabler-folder-off" text="还没有 Profile" />
        <div v-else class="grid gap-4 lg:grid-cols-2">
          <div v-for="profile in profiles" :key="`${profile.siteKey}:${profile.profileKey}`" class="rounded-lg border border-default bg-default">
            <div class="flex items-start justify-between gap-3 border-b border-default p-4">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <UIcon name="i-tabler-folder-cog" class="size-4 text-primary" />
                  <h3 class="truncate text-sm font-semibold text-highlighted">{{ profile.profileKey }}</h3>
                </div>
                <p class="mt-1 text-xs text-muted">{{ siteName(profile.siteKey) }} · {{ profile.purpose || '未填写用途' }}</p>
              </div>
              <div class="flex items-center gap-2">
                <UBadge :label="`${profile.assetCount} 素材`" color="neutral" variant="soft" />
                <UDropdownMenu :items="profileActions(profile)">
                  <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="xs" />
                </UDropdownMenu>
              </div>
            </div>
            <div class="space-y-3 p-4 text-sm">
              <div class="grid grid-cols-2 gap-3 text-xs text-muted">
                <div>类型 <span class="text-default">{{ profile.allowedExt }}</span></div>
                <div>上限 <span class="text-default">{{ formatBytes(profile.maxSizeBytes) }}</span></div>
                <div>后端 <span class="text-default">{{ profileStorageText(profile) }}</span></div>
                <div>访问级别 <span class="text-default">{{ profileAccessText(profile) }}</span></div>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-muted">Variant · {{ profile.variantCount }}</span>
                <UButton icon="i-tabler-plus" label="添加" color="neutral" variant="soft" size="xs" @click="editVariant(undefined, profile)" />
              </div>
              <div v-if="!variantsFor(profile).length" class="rounded-md border border-dashed border-default px-3 py-2 text-xs text-muted">还没有派生规格。</div>
              <div v-else class="space-y-1.5">
                <div
                  v-for="variant in variantsFor(profile)"
                  :key="variant.id"
                  class="flex items-center justify-between rounded-md bg-elevated/40 px-3 py-2 text-xs"
                >
                  <span class="font-medium text-default">{{ variant.variantKey }}</span>
                  <span class="text-muted">{{ variant.width }}x{{ variant.height }} · {{ variant.mode }} · v{{ variant.version }}</span>
                  <UDropdownMenu :items="variantActions(variant)">
                    <UButton icon="i-tabler-dots" color="neutral" variant="ghost" size="xs" />
                  </UDropdownMenu>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="tab === 'grants'" class="space-y-4">
        <ManageEmpty v-if="!grants.length" icon="i-tabler-key-off" text="还没有交付授权" />
        <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
          <div v-for="grant in grants" :key="grant.id" class="flex items-center gap-4 border-b border-default px-4 py-3 last:border-b-0">
            <span class="grid size-10 place-items-center rounded-lg bg-primary/10 text-primary">
              <UIcon name="i-tabler-key" class="size-5" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-highlighted">{{ grant.reason || grant.policy }}</div>
              <div class="mt-0.5 truncate font-mono text-xs text-muted">{{ grant.assetId }}</div>
            </div>
            <div class="hidden text-right text-xs text-muted sm:block">
              <div>{{ grant.usedCount }} / {{ grant.maxUses }}</div>
              <div>过期 {{ briefDate(grant.expiresAt) }}</div>
              <div>{{ grantLastUsedText(grant) }}</div>
              <div class="truncate">{{ grantAuditText(grant) }}</div>
            </div>
            <UBadge :label="grantStatus(grant).label" :color="grantStatus(grant).color" variant="soft" />
            <UBadge :label="grantPurposeLabel(grant)" color="info" variant="soft" />
            <UBadge :label="grant.policy" color="neutral" variant="soft" />
            <UDropdownMenu :items="grantActions(grant)">
              <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="xs" />
            </UDropdownMenu>
          </div>
        </div>
        <div class="flex items-center justify-between text-sm text-muted">
          <span>共 {{ totalGrants }} 条授权</span>
          <div class="flex items-center gap-2">
            <UButton icon="i-tabler-chevron-left" color="neutral" variant="ghost" :disabled="grantPage <= 1" @click="() => { grantPage-- }" />
            <span>{{ grantPage }}</span>
            <UButton icon="i-tabler-chevron-right" color="neutral" variant="ghost" :disabled="grantPage * GRANT_PAGE_SIZE >= totalGrants" @click="() => { grantPage++ }" />
          </div>
        </div>
      </section>
    </template>

    <UModal v-model:open="profileOpen" title="Profile 配置">
      <template #body>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="站点"><USelectMenu v-model="profileForm.siteKey" :items="siteOptions.filter(i => i.value !== ALL)" value-key="value" class="w-full" /></UFormField>
          <UFormField label="Profile Key"><UInput v-model="profileForm.profileKey" placeholder="blog-cover" /></UFormField>
          <UFormField label="用途" class="sm:col-span-2"><UInput v-model="profileForm.purpose" placeholder="文章封面 / 正文图片 / 付费资源" /></UFormField>
          <UFormField label="存储后端" class="sm:col-span-2">
            <USelectMenu
              v-model="profileForm.storageBackend"
              :items="profileStorageBackendOptions"
              value-key="value"
              class="w-full"
              :search-input="{ placeholder: '搜索后端…' }"
            />
          </UFormField>
          <UFormField label="允许后缀"><UInput v-model="profileForm.allowedExt" placeholder="jpg,jpeg,png,webp" /></UFormField>
          <UFormField label="大小上限(bytes)"><UInput v-model.number="profileForm.maxSizeBytes" type="number" /></UFormField>
          <UFormField label="访问级别" help="公开资源返回稳定公开地址；私有资源由业务授权后生成签名链接。">
            <USelect v-model="profileForm.defaultVisibility" :items="profileAccessLevelOptions" value-key="value" />
          </UFormField>
          <UCheckbox v-model="profileForm.keepOriginal" label="保留原图/原文件" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton label="取消" color="neutral" variant="ghost" @click="() => { profileOpen = false }" />
          <UButton label="保存" icon="i-tabler-device-floppy" @click="saveProfile" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="siteOpen" title="站点配置">
      <template #body>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Site Key">
            <UInput v-model="siteForm.siteKey" placeholder="blog" />
          </UFormField>
          <UFormField label="站点名称">
            <UInput v-model="siteForm.name" placeholder="Blog" />
          </UFormField>
          <UFormField label="默认存储后端">
            <USelectMenu
              v-model="siteForm.defaultStorageBackend"
              :items="storageBackendOptions"
              value-key="value"
              class="w-full"
              :search-input="{ placeholder: '搜索后端…' }"
            />
          </UFormField>
          <UFormField label="状态">
            <USwitch v-model="siteForm.enabled" label="允许上传" />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton label="取消" color="neutral" variant="ghost" @click="() => { siteOpen = false }" />
          <UButton label="保存" icon="i-tabler-device-floppy" @click="saveSite" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="storageBackendOpen" title="S3-compatible 存储后端">
      <template #body>
        <div class="space-y-4">
          <div v-if="storageBackendEditingName" class="rounded-lg border border-default bg-elevated/30 p-3">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="truncate text-sm font-medium text-highlighted">{{ storageBackendEditingName }}</span>
                  <UBadge :label="storageBackendHealthBadge(storageBackendDetail).label" :color="storageBackendHealthBadge(storageBackendDetail).color" variant="soft" size="sm" />
                </div>
                <div class="mt-1 text-xs text-muted">
                  Secret v{{ storageBackendDetail?.secretVersion || 1 }}
                  <span v-if="storageBackendDetail?.secretRotatedAt"> · 轮换 {{ briefDate(storageBackendDetail.secretRotatedAt) }}</span>
                  <span v-if="storageBackendDetail?.lastHealthCheckedAt"> · 检查 {{ briefDate(storageBackendDetail.lastHealthCheckedAt) }}</span>
                </div>
                <p v-if="storageBackendDetail?.lastHealthError" class="mt-1 line-clamp-2 text-xs text-error">{{ storageBackendDetail.lastHealthError }}</p>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <UButton icon="i-tabler-heartbeat" label="健康检查" color="neutral" variant="soft" size="xs" :loading="checkingStorageBackend" @click="checkStorageBackendHealth" />
                <UButton icon="i-tabler-key" label="轮换密钥" color="neutral" variant="soft" size="xs" :disabled="loadingStorageBackend" @click="() => { rotateStorageBackendOpen = true }" />
              </div>
            </div>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField label="后端名称">
              <UInput v-model="storageBackendForm.name" placeholder="s3main" :disabled="!!storageBackendEditingName || loadingStorageBackend" />
            </UFormField>
            <UFormField label="后端类型">
              <USelectMenu v-model="storageBackendForm.type" :items="storageBackendTypeItems" value-key="value" class="w-full" :disabled="!!storageBackendEditingName || loadingStorageBackend" />
            </UFormField>
            <UFormField label="Region">
              <UInput v-model="storageBackendForm.region" placeholder="us-east-1" />
            </UFormField>
            <UFormField label="Endpoint">
              <UInput v-model="storageBackendForm.endpoint" placeholder="localhost:9000" />
            </UFormField>
            <UFormField label="公开访问 Base URL">
              <UInput v-model="storageBackendForm.publicBaseUrl" placeholder="https://cdn.example.com" />
            </UFormField>
            <UFormField label="Public Bucket">
              <UInput v-model="storageBackendForm.bucketPublic" placeholder="asset-public" />
            </UFormField>
            <UFormField label="Private Bucket">
              <UInput v-model="storageBackendForm.bucketPrivate" placeholder="asset-private" />
            </UFormField>
            <UFormField label="Access Key">
              <UInput v-model="storageBackendForm.accessKey" autocomplete="off" />
            </UFormField>
            <UFormField label="Secret Key">
              <UInput v-model="storageBackendForm.secretKey" type="password" autocomplete="new-password" :placeholder="storageBackendEditingName ? '留空沿用已有密钥' : ''" />
            </UFormField>
            <UCheckbox v-model="storageBackendForm.pathStyle" label="Path-style endpoint" />
            <UCheckbox v-model="storageBackendForm.useSsl" label="使用 HTTPS" />
            <UCheckbox v-model="storageBackendForm.enabled" label="启用后端" />
            <p class="text-xs text-muted sm:col-span-2">
              保存时会先连接并注册后端；如果配置不可用，不会写入运行时。编辑已有后端时 Secret Key 留空会沿用旧密钥。
            </p>
          </div>

          <div v-if="storageBackendEditingName" class="rounded-lg border border-default bg-default">
            <div class="flex items-center justify-between border-b border-default px-3 py-2">
              <h3 class="text-xs font-medium text-muted">最近事件</h3>
              <UButton icon="i-tabler-refresh" color="neutral" variant="ghost" square size="xs" @click="fetchStorageBackendEvents()" />
            </div>
            <ManageEmpty v-if="!storageBackendEvents.length" icon="i-tabler-history" text="还没有后端事件" />
            <div v-else class="max-h-56 divide-y divide-default overflow-y-auto">
              <div v-for="event in storageBackendEvents" :key="event.id" class="px-3 py-2.5">
                <div class="flex min-w-0 items-center gap-2">
                  <span class="truncate text-sm font-medium text-highlighted">{{ storageBackendEventTypeLabel(event.eventType) }}</span>
                  <UBadge :label="event.status" :color="storageBackendEventStatusColor(event.status)" variant="soft" size="sm" />
                  <span class="ml-auto shrink-0 text-xs text-muted">{{ briefDate(event.createdAt) }}</span>
                </div>
                <div class="mt-0.5 truncate text-xs text-muted">{{ event.actor || 'system' }}<span v-if="event.message"> · {{ event.message }}</span></div>
              </div>
            </div>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full items-center justify-between gap-2">
          <UButton
            v-if="storageBackendEditingName"
            label="删除后端"
            icon="i-tabler-trash"
            color="error"
            variant="ghost"
            :disabled="savingStorageBackend"
            @click="() => { deleteStorageBackendOpen = true }"
          />
          <span v-else />
          <div class="flex items-center gap-2">
            <UButton label="取消" color="neutral" variant="ghost" :disabled="savingStorageBackend" @click="() => { storageBackendOpen = false }" />
            <UButton label="保存" icon="i-tabler-device-floppy" :loading="savingStorageBackend" @click="saveStorageBackend" />
          </div>
        </div>
      </template>
    </UModal>

    <UModal v-model:open="deleteStorageBackendOpen" title="删除存储后端?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ storageBackendEditingName }}</span>。只有没有站点默认使用、没有 Profile 指定、也没有素材落在该后端时才允许删除。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingStorageBackend" @click="() => { deleteStorageBackendOpen = false }" />
          <UButton color="error" label="确认删除" :loading="deletingStorageBackend" @click="confirmDeleteStorageBackend" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="rotateStorageBackendOpen" title="轮换存储后端密钥">
      <template #body>
        <div class="space-y-3">
          <p class="text-sm text-muted">
            为 <span class="font-medium text-default">{{ storageBackendEditingName }}</span> 写入新的 Secret Key，并记录一次密钥轮换事件。
          </p>
          <UFormField label="新的 Secret Key">
            <UInput v-model="rotateStorageBackendSecret" type="password" autocomplete="new-password" autofocus />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="rotatingStorageBackend" @click="() => { rotateStorageBackendOpen = false }" />
          <UButton color="primary" label="确认轮换" icon="i-tabler-key" :loading="rotatingStorageBackend" :disabled="!rotateStorageBackendSecret" @click="confirmRotateStorageBackendSecret" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="variantOpen" title="Variant 规则">
      <template #body>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="站点"><UInput v-model="variantForm.siteKey" disabled /></UFormField>
          <UFormField label="Profile"><UInput v-model="variantForm.profileKey" disabled /></UFormField>
          <UFormField label="Variant Key"><UInput v-model="variantForm.variantKey" placeholder="card / og / content" /></UFormField>
          <UFormField label="模式"><USelect v-model="variantForm.mode" :items="modeOptions" value-key="value" /></UFormField>
          <UFormField label="宽度"><UInput v-model.number="variantForm.width" type="number" /></UFormField>
          <UFormField label="高度"><UInput v-model.number="variantForm.height" type="number" /></UFormField>
          <UFormField label="质量"><UInput v-model.number="variantForm.quality" type="number" /></UFormField>
          <UFormField label="版本"><UInput v-model.number="variantForm.version" type="number" /></UFormField>
          <UCheckbox v-model="variantForm.enabled" label="启用" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton label="取消" color="neutral" variant="ghost" @click="() => { variantOpen = false }" />
          <UButton label="保存" icon="i-tabler-device-floppy" @click="saveVariant" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="deleteProfileOpen" title="删除 Profile?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ deleteProfileTarget?.profileKey }}</span>。只有没有素材、没有 Variant 的 Profile 才允许删除。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingProfile" @click="() => { deleteProfileTarget = null }" />
          <UButton color="error" label="确认删除" :loading="deletingProfile" @click="confirmDeleteProfile" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="deleteAssetOpen" title="删除素材?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ deleteAssetTarget?.filename || deleteAssetTarget?.id }}</span>。原文件、派生图和相关交付授权会一并失效。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingAsset" @click="() => { deleteAssetTarget = null }" />
          <UButton color="error" label="确认删除" :loading="deletingAsset" @click="confirmDeleteAsset" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="pruneOpen" title="清理无引用素材">
      <template #body>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <UFormField label="保留天数">
              <UInput v-model.number="pruneForm.olderThanDays" type="number" min="1" />
            </UFormField>
            <UFormField label="单次上限">
              <UInput v-model.number="pruneForm.limit" type="number" min="1" max="200" />
            </UFormField>
          </div>
          <p class="text-sm text-muted">
            将只处理超过 {{ pruneForm.olderThanDays }} 天、且没有任何业务引用的素材。删除前后端仍会再次检查引用。
          </p>
          <div class="rounded-lg border border-default bg-default">
            <div class="border-b border-default px-3 py-2 text-sm text-muted">
              候选 {{ prunePreview?.candidates ?? 0 }} 个，本次最多处理 {{ pruneForm.limit }} 个
            </div>
            <ManageEmpty v-if="!prunePreview?.items?.length" icon="i-tabler-unlink" text="没有可清理的无引用素材" />
            <div v-else class="max-h-72 overflow-auto">
              <div v-for="asset in prunePreview.items" :key="asset.id" class="flex items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
                <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
                  <UIcon name="i-tabler-file" class="size-4" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium text-highlighted">{{ asset.filename || asset.id }}</div>
                  <div class="truncate text-xs text-muted">{{ asset.spaceKey || 'default' }} / {{ asset.siteKey }} / {{ asset.profileKey }} · {{ formatBytes(asset.size) }} · {{ briefDate(asset.createdAt) }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="pruningUnreferenced" @click="() => { pruneOpen = false }" />
          <UButton
            color="error"
            label="确认清理"
            icon="i-tabler-trash"
            :loading="pruningUnreferenced"
            :disabled="!prunePreview?.items?.length"
            @click="confirmPruneUnreferenced"
          />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="orphanObjectsOpen" title="扫描孤儿对象">
      <template #body>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-3">
            <UFormField label="存储后端">
              <USelectMenu
                v-model="orphanObjectsForm.backend"
                :items="[{ label: '全部后端', value: ALL }, ...storageBackendOptions]"
                value-key="value"
                class="w-full"
              />
            </UFormField>
            <UFormField label="保留天数">
              <UInput v-model.number="orphanObjectsForm.olderThanDays" type="number" min="1" />
            </UFormField>
            <UFormField label="返回上限">
              <UInput v-model.number="orphanObjectsForm.limit" type="number" min="1" max="500" />
            </UFormField>
          </div>
          <p class="text-sm text-muted">
            会扫描 public/private 对象,和数据库里的原文件及当前派生图 key 对比。只处理超过 {{ orphanObjectsForm.olderThanDays }} 天的多余对象。
          </p>
          <div class="space-y-3">
            <div
              v-for="backend in orphanObjectsPreview?.items ?? []"
              :key="backend.backend"
              class="rounded-lg border border-default bg-default"
            >
              <div class="flex flex-wrap items-center justify-between gap-2 border-b border-default px-3 py-2">
                <div class="text-sm font-medium text-highlighted">{{ backend.backend }}</div>
                <div class="flex flex-wrap items-center gap-2">
                  <UBadge :label="backend.skipped ? '不支持扫描' : `扫描 ${backend.scanned}`" :color="backend.skipped ? 'neutral' : 'primary'" variant="soft" />
                  <UBadge :label="`期望 ${backend.expected}`" color="neutral" variant="soft" />
                  <UBadge :label="`孤儿 ${backend.orphans}`" :color="backend.orphans ? 'warning' : 'success'" variant="soft" />
                </div>
              </div>
              <div v-if="backend.error" class="px-3 py-2 text-sm text-error">{{ backend.error }}</div>
              <ManageEmpty v-else-if="!backend.items?.length" icon="i-tabler-database-check" text="没有发现可清理对象" />
              <div v-else class="max-h-64 overflow-auto">
                <div v-for="item in backend.items" :key="item.key" class="flex items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
                  <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
                    <UIcon name="i-tabler-file-database" class="size-4" />
                  </span>
                  <div class="min-w-0 flex-1">
                    <div class="truncate font-mono text-xs text-highlighted">{{ item.key }}</div>
                    <div class="mt-0.5 text-xs text-muted">{{ formatBytes(item.size) }} · {{ briefDate(item.modTime || '') }}</div>
                  </div>
                </div>
              </div>
            </div>
            <ManageEmpty v-if="!(orphanObjectsPreview?.items?.length)" icon="i-tabler-database-search" text="还没有扫描结果" />
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="auditingOrphanObjects" @click="() => { orphanObjectsOpen = false }" />
          <UButton
            color="error"
            label="确认清理"
            icon="i-tabler-trash"
            :loading="auditingOrphanObjects"
            :disabled="!(orphanObjectsPreview?.orphans)"
            @click="confirmPruneOrphanObjects"
          />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="storageMigrationOpen" title="迁移存储后端">
      <template #body>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-3">
            <UFormField label="源后端">
              <USelectMenu v-model="storageMigrationForm.sourceBackend" :items="storageBackendOptions" value-key="value" class="w-full" />
            </UFormField>
            <UFormField label="目标后端">
              <USelectMenu v-model="storageMigrationForm.targetBackend" :items="storageBackendOptions" value-key="value" class="w-full" />
            </UFormField>
            <UFormField label="单次上限">
              <UInput v-model.number="storageMigrationForm.limit" type="number" min="1" max="200" />
            </UFormField>
          </div>
          <p class="text-sm text-muted">
            会把源后端中的原文件和当前派生图复制到目标后端，成功后更新素材所属后端。旧后端对象不会立即删除，可再用孤儿对象扫描清理。
          </p>
          <div class="rounded-lg border border-default bg-default">
            <div class="border-b border-default px-3 py-2 text-sm text-muted">
              候选 {{ storageMigrationPreview?.candidates ?? 0 }} 个，本次最多处理 {{ storageMigrationForm.limit }} 个
            </div>
            <ManageEmpty v-if="!storageMigrationPreview?.items?.length" icon="i-tabler-database-off" text="还没有预检结果" />
            <div v-else class="max-h-72 overflow-auto">
              <div v-for="asset in storageMigrationPreview.items" :key="asset.id" class="flex items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
                <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
                  <UIcon name="i-tabler-file-database" class="size-4" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium text-highlighted">{{ asset.filename || asset.id }}</div>
                  <div class="truncate text-xs text-muted">{{ asset.storageBackend }} · {{ asset.mime }} · {{ formatBytes(asset.size) }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="migratingStorage" @click="() => { storageMigrationOpen = false }" />
          <UButton
            color="neutral"
            variant="soft"
            label="预检"
            icon="i-tabler-search"
            :loading="migratingStorage"
            :disabled="!storageMigrationForm.sourceBackend || !storageMigrationForm.targetBackend || storageMigrationForm.sourceBackend === storageMigrationForm.targetBackend"
            @click="previewStorageMigration"
          />
          <UButton
            label="确认迁移"
            icon="i-tabler-transfer"
            :loading="migratingStorage"
            :disabled="!storageMigrationPreview?.items?.length || storageMigrationForm.sourceBackend === storageMigrationForm.targetBackend"
            @click="confirmStorageMigration"
          />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="referencesOpen" title="素材引用">
      <template #body>
        <div class="space-y-3">
          <p class="truncate font-mono text-xs text-dimmed">{{ referenceAsset?.id }}</p>
          <SkeletonList v-if="loadingReferences" :rows="3" />
          <ManageEmpty v-else-if="!references.length" icon="i-tabler-link-off" text="还没有引用记录" />
          <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
            <div v-for="ref in references" :key="ref.id" class="flex items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
              <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
                <UIcon name="i-tabler-link" class="size-4" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium text-highlighted">{{ ref.refLabel || ref.refId }}</div>
                <div class="truncate text-xs text-muted">{{ ref.siteKey }} · {{ ref.refType }} · {{ ref.refId }}</div>
              </div>
              <UButton
                v-if="ref.refUrl"
                icon="i-tabler-external-link"
                color="neutral"
                variant="ghost"
                square
                size="xs"
                @click="openExternal(ref.refUrl)"
              />
            </div>
          </div>
        </div>
      </template>
    </UModal>

    <UModal v-model:open="deleteVariantOpen" title="删除 Variant?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ deleteVariantTarget?.variantKey }}</span>。已有文件不会被删除，但之后不会再生成这个派生规格。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingVariant" @click="() => { deleteVariantTarget = null }" />
          <UButton color="error" label="确认删除" :loading="deletingVariant" @click="confirmDeleteVariant" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="batchRebuildOpen" title="批量重建派生图">
      <template #body>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-[1fr_140px]">
            <UFormField label="Profile">
              <UInput :model-value="batchRebuildProfile ? `${batchRebuildProfile.siteKey} / ${batchRebuildProfile.profileKey}` : ''" disabled />
            </UFormField>
            <UFormField label="单次上限">
              <UInput v-model.number="batchRebuildLimit" type="number" min="1" max="200" />
            </UFormField>
          </div>
          <p class="text-sm text-muted">
            会删除并重新生成该 Profile 下图片素材的当前 Variant。建议先小批量执行，确认规则无误后再继续。
          </p>
          <div class="rounded-lg border border-default bg-default">
            <div class="border-b border-default px-3 py-2 text-sm text-muted">
              候选 {{ batchRebuildPreview?.candidates ?? 0 }} 个，本次最多处理 {{ batchRebuildLimit }} 个
            </div>
            <ManageEmpty v-if="!batchRebuildPreview?.items?.length" icon="i-tabler-photo-off" text="没有可重建的图片素材" />
            <div v-else class="max-h-72 overflow-auto">
              <div v-for="asset in batchRebuildPreview.items" :key="asset.id" class="flex items-center gap-3 border-b border-default px-3 py-2.5 last:border-b-0">
                <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
                  <UIcon name="i-tabler-photo" class="size-4" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-medium text-highlighted">{{ asset.filename || asset.id }}</div>
                  <div class="truncate text-xs text-muted">{{ asset.mime }} · {{ formatBytes(asset.size) }} · {{ briefDate(asset.createdAt) }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="batchRebuilding" @click="() => { batchRebuildOpen = false }" />
          <UButton
            color="primary"
            label="确认重建"
            icon="i-tabler-refresh-dot"
            :loading="batchRebuilding"
            :disabled="!batchRebuildPreview?.items?.length"
            @click="confirmBatchRebuild"
          />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="createGrantOpen" title="签发交付链接">
      <template #body>
        <div class="space-y-4">
          <p class="truncate font-mono text-xs text-dimmed">{{ grantAsset?.id }}</p>
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField label="Variant">
              <UInput v-model="grantForm.variantKey" placeholder="original / card / cover" />
            </UFormField>
            <UFormField label="策略">
              <USelect v-model="grantForm.policy" :items="policyOptions" value-key="value" />
            </UFormField>
            <UFormField label="有效期(秒)">
              <UInput v-model.number="grantForm.expiresIn" type="number" min="60" />
            </UFormField>
            <UFormField label="最大使用次数">
              <UInput v-model.number="grantForm.maxUses" type="number" min="1" />
            </UFormField>
            <UFormField label="Subject ID" class="sm:col-span-2">
              <UInput v-model="grantForm.subjectId" placeholder="留空表示不绑定用户" />
            </UFormField>
            <UFormField label="原因" class="sm:col-span-2">
              <UInput v-model="grantForm.reason" />
            </UFormField>
          </div>
          <p class="text-xs text-muted">有效期最多 24 小时，最大使用次数最多 100 次；服务端会自动收紧超出的值。</p>
          <div v-if="createdGrant" class="rounded-lg border border-default bg-default p-3">
            <div class="mb-2 flex items-center justify-between gap-2">
              <div class="text-sm font-medium text-highlighted">交付链接</div>
              <UButton icon="i-tabler-copy" label="复制" color="neutral" variant="soft" size="xs" @click="copyCreatedGrant" />
            </div>
            <p class="break-all font-mono text-xs text-muted">{{ createdGrant.url }}</p>
            <p class="mt-2 text-xs text-muted">过期 {{ briefDate(createdGrant.expiresAt) }}</p>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="关闭" :disabled="creatingGrant" @click="() => { createGrantOpen = false }" />
          <UButton label="生成链接" icon="i-tabler-key" :loading="creatingGrant" @click="createGrant" />
        </div>
      </template>
    </UModal>

    <UModal v-model:open="revokeGrantOpen" title="撤销授权?">
      <template #body>
        <p class="text-sm text-muted">
          将撤销这条交付授权。撤销后，已发出去的一次性或门禁链接会立即失效。
        </p>
        <p class="mt-2 truncate font-mono text-xs text-dimmed">{{ revokeGrantTarget?.assetId }}</p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="revokingGrant" @click="() => { revokeGrantTarget = null }" />
          <UButton color="error" label="确认撤销" :loading="revokingGrant" @click="confirmRevokeGrant" />
        </div>
      </template>
    </UModal>
  </div>
</template>
