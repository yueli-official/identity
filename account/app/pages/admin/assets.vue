<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import {
  ManageEmpty,
  ManageHeader,
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
import type {
  AssetAdminSection,
  AssetAdminStats as Stats,
  AssetBatchRebuildResult as BatchRebuildResult,
  AssetGrant as Grant,
  AssetGrantForm as GrantForm,
  AssetItem,
  AssetMaintenanceTask as MaintenanceTask,
  AssetOrphanObjectBackend as OrphanObjectBackend,
  AssetOrphanObjectItem as OrphanObjectItem,
  AssetOrphanObjectResult as OrphanObjectResult,
  AssetProfile as Profile,
  AssetPruneResult as PruneResult,
  AssetReference,
  AssetSite as Site,
  AssetSpaceUsage,
  AssetStorageBackend as StorageBackend,
  AssetStorageBackendDetail as StorageBackendDetail,
  AssetStorageBackendEvent as StorageBackendEvent,
  AssetStorageBackendForm as StorageBackendForm,
  AssetStorageMigrationResult as StorageMigrationResult,
  AssetSweepResult as SweepResult,
  AssetVariant as Variant,
  CreatedAssetGrant as CreatedGrant
} from '~/types/asset-admin'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '资源管理 · 控制台' })

const { call } = useApi()
const toast = useToast()
const route = useRoute()
const router = useRouter()
const ALL = '__all__'

const mounted = ref(false)
const loading = ref(true)
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

const selectionResetKey = computed(() => manageCollectionQueryFingerprint(
  serializeManageCollectionQuery(collectionState.value, collectionDefinition)
))
const {
  selectedIds,
  isPageSelected,
  isPageIndeterminate,
  toggleOne,
  togglePage,
  clear: clearSelection
} = useManageSelection({
  visibleIds: computed(() => assets.value.map(asset => asset.id)),
  filteredTotal: computed(() => totalAssets.value),
  resetKey: selectionResetKey
})

const selectedTask = computed(() => maintenanceTasks.value.find(task => task.id === selectedTaskId.value))

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
const savingProfile = ref(false)
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
async function saveProfile(value: Profile) {
  Object.assign(profileForm, value)
  if (profileForm.defaultVisibility === 'public') {
    profileForm.defaultDeliveryPolicy = 'public'
  } else if (profileForm.defaultDeliveryPolicy !== 'signed') {
    profileForm.defaultDeliveryPolicy = 'signed'
  }
  savingProfile.value = true
  try {
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
  } finally {
    savingProfile.value = false
  }
}

const siteOpen = ref(false)
const savingSite = ref(false)
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
async function saveSite(value: Site) {
  Object.assign(siteForm, value)
  savingSite.value = true
  try {
    await call('/api/v1/admin/assets-proxy/sites', { method: 'POST', body: siteForm })
    toast.add({ title: '站点已保存', color: 'success', icon: 'i-tabler-check' })
    siteOpen.value = false
    await reloadAll()
  } finally {
    savingSite.value = false
  }
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
const deleteStorageBackendOpen = ref(false)
const deletingStorageBackend = ref(false)
const storageBackendForm = reactive<StorageBackendForm>({
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
async function saveStorageBackend(value: StorageBackendForm) {
  Object.assign(storageBackendForm, value)
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
async function confirmRotateStorageBackendSecret(secret: string) {
  if (!storageBackendEditingName.value || !secret) return
  rotatingStorageBackend.value = true
  try {
    const data = await call<{ backend: StorageBackendDetail }>(
      `/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(storageBackendEditingName.value)}/rotate-secret`,
      { method: 'POST', body: { secretKey: secret } }
    )
    storageBackendDetail.value = data.backend
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
const savingVariant = ref(false)
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
async function saveVariant(value: Variant) {
  Object.assign(variantForm, value)
  savingVariant.value = true
  try {
    await call('/api/v1/admin/assets-proxy/variants', { method: 'POST', body: variantForm })
    toast.add({ title: 'Variant 已保存', color: 'success', icon: 'i-tabler-check' })
    variantOpen.value = false
    await reloadAll()
  } finally {
    savingVariant.value = false
  }
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
const grantForm = reactive<GrantForm>({
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

async function createGrant(value: GrantForm) {
  if (!grantAsset.value) return
  Object.assign(grantForm, value)
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
function canRevokeGrant(grant: Grant) {
  return grantStatus(grant).label === '有效'
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
      <AssetLibraryPanel
        v-if="tab === 'library'"
        v-model:search="searchInput"
        v-model:space-key="spaceKey"
        v-model:site-key="siteKey"
        v-model:profile-key="profileKey"
        v-model:visibility="visibility"
        v-model:mime="mime"
        v-model:sort="sort"
        v-model:direction="direction"
        v-model:view="libraryView"
        v-model:page="page"
        v-model:page-size="size"
        :assets="assets"
        :total="totalAssets"
        :filter-count="activeLibraryFilterCount"
        :space-options="spaceOptions"
        :site-options="siteOptions"
        :profile-options="profileOptions"
        :visibility-options="visibilityOptions"
        :mime-options="mimeOptions"
        :sort-options="sortOptions"
        :selected-ids="selectedIds"
        :page-selected="isPageSelected"
        :page-indeterminate="isPageIndeterminate"
        :page-size-items="pageSizeItems"
        :total-pages="totalAssetPages"
        :sites="sites"
        :queueing="queueingSelectedRebuild"
        :queue-error="selectedRebuildError"
        :task-id="selectedTaskId"
        :task="selectedTask"
        :controlling-task-id="controllingMaintenanceTaskId"
        :actions-for="assetActions"
        @toggle-one="toggleOne"
        @toggle-page="togglePage"
        @queue-selected="queueSelectedRebuild"
        @clear-selection="clearSelection"
        @task-action="controlSelectedTask"
        @open-maintenance="openMaintenanceTaskList"
        @dismiss-task="dismissSelectedTask"
      />

      <AssetStoragePanel v-else-if="tab === 'storage'" :backends="storageBackends" @edit="editStorageBackend" />

      <AssetMaintenancePanel
        v-else-if="tab === 'maintenance'"
        :tasks="maintenanceTasks"
        :controlling-task-id="controllingMaintenanceTaskId"
        :sweeping="sweepingStaging"
        :pruning="pruningUnreferenced"
        :auditing="auditingOrphanObjects"
        @refresh="fetchMaintenanceTasks"
        @sweep-staging="sweepStaging"
        @preview-prune="previewPruneUnreferenced"
        @preview-orphans="previewOrphanObjects"
        @control="controlMaintenanceTask"
      />

      <AssetSitesPanel v-else-if="tab === 'sites'" :sites="sites" @edit="editSite" />

      <AssetProfilesPanel
        v-else-if="tab === 'profiles'"
        :profiles="profiles"
        :variants="variants"
        :sites="sites"
        :profile-actions="profileActions"
        :variant-actions="variantActions"
        @add-variant="profile => editVariant(undefined, profile)"
      />

      <AssetGrantsPanel
        v-else-if="tab === 'grants'"
        v-model:page="grantPage"
        :grants="grants"
        :total="totalGrants"
        :page-size="GRANT_PAGE_SIZE"
        :actions-for="grantActions"
      />
    </template>

    <AssetProfileModal
      v-model:open="profileOpen"
      :initial-value="profileForm"
      :sites="sites"
      :site-options="siteOptions.filter(item => item.value !== ALL)"
      :storage-backend-options="storageBackendOptions"
      :access-level-options="profileAccessLevelOptions"
      :saving="savingProfile"
      @save="saveProfile"
    />

    <AssetSiteModal
      v-model:open="siteOpen"
      :initial-value="siteForm"
      :storage-backend-options="storageBackendOptions"
      :saving="savingSite"
      @save="saveSite"
    />

    <AssetStorageBackendModal
      v-model:open="storageBackendOpen"
      :initial-value="storageBackendForm"
      :editing-name="storageBackendEditingName"
      :detail="storageBackendDetail"
      :events="storageBackendEvents"
      :type-options="storageBackendTypeItems"
      :loading="loadingStorageBackend"
      :saving="savingStorageBackend"
      :checking="checkingStorageBackend"
      @save="saveStorageBackend"
      @check-health="checkStorageBackendHealth"
      @refresh-events="fetchStorageBackendEvents()"
      @rotate-secret="rotateStorageBackendOpen = true"
      @delete="deleteStorageBackendOpen = true"
    />

    <AssetStorageBackendDeleteModal
      v-model:open="deleteStorageBackendOpen"
      :backend-name="storageBackendEditingName"
      :deleting="deletingStorageBackend"
      @confirm="confirmDeleteStorageBackend"
    />

    <AssetStorageBackendRotateModal
      v-model:open="rotateStorageBackendOpen"
      :backend-name="storageBackendEditingName"
      :rotating="rotatingStorageBackend"
      @confirm="confirmRotateStorageBackendSecret"
    />

    <AssetVariantModal
      v-model:open="variantOpen"
      :initial-value="variantForm"
      :mode-options="modeOptions"
      :saving="savingVariant"
      @save="saveVariant"
    />

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

    <AssetGrantModal
      v-model:open="createGrantOpen"
      :asset-id="grantAsset?.id"
      :initial-value="grantForm"
      :created-grant="createdGrant"
      :policy-options="policyOptions"
      :creating="creatingGrant"
      @create="createGrant"
    />

    <AssetRevokeGrantModal
      v-model:open="revokeGrantOpen"
      :asset-id="revokeGrantTarget?.assetId"
      :revoking="revokingGrant"
      @confirm="confirmRevokeGrant"
    />
  </div>
</template>
