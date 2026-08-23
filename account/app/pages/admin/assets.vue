<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import { SkeletonList } from '~/components/manage'
import { createCollectionRouteQueryCodec, createJsonCollectionQueryPolicy, type CollectionPanelState, type CollectionWorkflow } from '@yueli/ui/collection'
import { useVueCollectionWorkflow } from '@yueli/ui/collection/vue'
import { createVueRouterCollectionQuerySync } from '@yueli/ui/collection/vue-router'
import { PageHeader, TabbedSurface } from '@yueli/ui/admin'
import { useMinimumLoading } from '@yueli/ui/feedback'
import type {
  AssetAdminSection,
  AssetConsumerRegistrationState,
  AssetGrant as Grant,
  AssetItem,
  AssetProfile as Profile,
  AssetSite as Site,
  AssetSpaceUsage,
  AssetStorageBackend as StorageBackend,
} from '~/types/asset-admin'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '资源管理 · 控制台' })

const { call } = useApi()
const router = useRouter()
const ALL = '__all__' as const

const mounted = ref(false)
const loading = ref(true)
const sectionReady = reactive<Record<Exclude<AssetAdminSection, 'library'>, boolean>>({
  registrations: false,
  storage: false,
  maintenance: false,
  grants: false,
})
const sectionLoading = reactive<Record<Exclude<AssetAdminSection, 'library'>, boolean>>({
  registrations: true,
  storage: true,
  maintenance: true,
  grants: true,
})
const sectionIssues = reactive<Record<AssetAdminSection, string>>({
  library: '',
  registrations: '',
  storage: '',
  maintenance: '',
  grants: '',
})
type AssetSort = 'createdAt' | 'filename' | 'size'
type AssetDirection = 'asc' | 'desc'
type AssetView = 'grid' | 'list'
interface AssetCollectionQuery {
  q: string
  sort: AssetSort
  direction: AssetDirection
  page: number
  size: number
  view: AssetView
  section: AssetAdminSection
  siteKey: string
  spaceKey: string
  profileKey: string
  visibility: string
  mime: string
  task: string
}
const sections = ['library', 'registrations', 'storage', 'maintenance', 'grants'] as const
const sorts = ['createdAt', 'filename', 'size'] as const
const views = ['grid', 'list'] as const
const pageSizes = [12, 24, 48, 96] as const
const defaultQuery: AssetCollectionQuery = {
  q: '',
  sort: 'createdAt',
  direction: 'desc',
  page: 1,
  size: 24,
  view: 'grid',
  section: 'library',
  siteKey: ALL,
  spaceKey: ALL,
  profileKey: ALL,
  visibility: ALL,
  mime: ALL,
  task: '',
}
const searchInput = ref('')
const queryPolicy = createJsonCollectionQueryPolicy<AssetCollectionQuery>()
const sync = createVueRouterCollectionQuerySync({
  router,
  codec: createCollectionRouteQueryCodec({
    q: { kind: 'string', default: defaultQuery.q, maxLength: 200 },
    sort: { kind: 'enum', values: sorts, default: defaultQuery.sort },
    direction: {
      kind: 'enum',
      values: ['asc', 'desc'] as const,
      default: defaultQuery.direction,
    },
    page: { kind: 'positive-integer', default: defaultQuery.page },
    size: {
      kind: 'positive-integer',
      values: pageSizes,
      default: defaultQuery.size,
    },
    view: { kind: 'enum', values: views, default: defaultQuery.view },
    section: { kind: 'enum', values: sections, default: defaultQuery.section },
    siteKey: { kind: 'string', default: defaultQuery.siteKey, maxLength: 200 },
    spaceKey: {
      kind: 'string',
      default: defaultQuery.spaceKey,
      maxLength: 200,
    },
    profileKey: {
      kind: 'string',
      default: defaultQuery.profileKey,
      maxLength: 200,
    },
    visibility: {
      kind: 'string',
      default: defaultQuery.visibility,
      maxLength: 100,
    },
    mime: { kind: 'string', default: defaultQuery.mime, maxLength: 200 },
    task: { kind: 'string', default: defaultQuery.task, maxLength: 200 },
  }),
})

async function loadAssets(nextQuery: Readonly<AssetCollectionQuery>, activeWorkflow: CollectionWorkflow<AssetItem, string, AssetCollectionQuery>) {
  const token = activeWorkflow.beginLoad()
  try {
    const data = await call<{ items: AssetItem[]; total: number }>('/api/v1/admin/assets-proxy/library', {
      params: {
        q: nextQuery.q || undefined,
        page: nextQuery.page,
        size: nextQuery.size,
        spaceKey: nextQuery.spaceKey !== ALL ? nextQuery.spaceKey : undefined,
        siteKey: nextQuery.siteKey !== ALL ? nextQuery.siteKey : undefined,
        profileKey: nextQuery.profileKey !== ALL ? nextQuery.profileKey : undefined,
        visibility: nextQuery.visibility !== ALL ? nextQuery.visibility : undefined,
        mime: nextQuery.mime !== ALL ? nextQuery.mime : undefined,
        sortBy: nextQuery.sort,
        sortOrder: nextQuery.direction,
      },
    })
    const lastPage = Math.max(1, Math.ceil(data.total / nextQuery.size))
    if (nextQuery.page > lastPage) {
      activeWorkflow.setQuery({ ...nextQuery, page: lastPage })
      return
    }
    activeWorkflow.resolveLoad(token, {
      items: data.items ?? [],
      total: data.total ?? 0,
    })
  } catch {
    activeWorkflow.rejectLoad(token, {
      key: 'account.assets.collection.load_failed',
    })
  }
}

const {
  snapshot: assetCollection,
  workflow: assetWorkflow,
  reload: reloadAssets,
} = useVueCollectionWorkflow({
  initialQuery: defaultQuery,
  queryPolicy,
  keyOf: (asset: AssetItem) => asset.id,
  querySync: sync,
  dataQueryKey: (query) =>
    JSON.stringify({
      ...query,
      view: undefined,
      section: undefined,
      task: undefined,
    }),
  load: loadAssets,
})
const collectionQuery = computed(() => assetCollection.value.query)
function updateCollectionQuery(patch: Partial<AssetCollectionQuery>, resetPage = true) {
  assetWorkflow.setQuery({
    ...collectionQuery.value,
    ...patch,
    ...(resetPage ? { page: 1 } : {}),
  })
}
const q = computed(() => collectionQuery.value.q)
const sort = computed({
  get: () => collectionQuery.value.sort,
  set: (value: AssetSort) => updateCollectionQuery({ sort: value }),
})
const direction = computed({
  get: () => collectionQuery.value.direction,
  set: (value: AssetDirection) => updateCollectionQuery({ direction: value }),
})
const page = computed({
  get: () => collectionQuery.value.page,
  set: (value: number) => updateCollectionQuery({ page: value }, false),
})
const size = computed({
  get: () => collectionQuery.value.size,
  set: (value: number) => updateCollectionQuery({ size: value }),
})
const libraryView = computed({
  get: () => collectionQuery.value.view,
  set: (value: AssetView) => updateCollectionQuery({ view: value }, false),
})
const section = computed({
  get: () => collectionQuery.value.section,
  set: (value: AssetAdminSection) => updateCollectionQuery({ section: value }, false),
})
const tab = computed<AssetAdminSection>({
  get: () => section.value,
  set: (value) => {
    section.value = value
  },
})
const siteKey = computed({
  get: () => collectionQuery.value.siteKey,
  set: (value: string) => updateCollectionQuery({ siteKey: value }),
})
const spaceKey = computed({
  get: () => collectionQuery.value.spaceKey,
  set: (value: string) => updateCollectionQuery({ spaceKey: value }),
})
const profileKey = computed({
  get: () => collectionQuery.value.profileKey,
  set: (value: string) => updateCollectionQuery({ profileKey: value }),
})
const visibility = computed({
  get: () => collectionQuery.value.visibility,
  set: (value: string) => updateCollectionQuery({ visibility: value }),
})
const mime = computed({
  get: () => collectionQuery.value.mime,
  set: (value: string) => updateCollectionQuery({ mime: value }),
})
const selectedTaskId = computed({
  get: () => collectionQuery.value.task,
  set: (value: string) => updateCollectionQuery({ task: value }, false),
})

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(searchInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => updateCollectionQuery({ q: value.trim() }), 300)
})
searchInput.value = assetCollection.value.query.q
watch(q, (value) => {
  if (searchInput.value !== value) searchInput.value = value
})
onScopeDispose(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
const spaces = ref<AssetSpaceUsage[]>([])
const registrationStates = ref<AssetConsumerRegistrationState[]>([])
const sites = computed<Site[]>(() => registrationStates.value.flatMap((state) => {
  const registration = state.registration
  if (!registration) return []
  return [{
    siteKey: registration.namespaceKey,
    name: registration.effective.displayName,
    defaultStorageBackend: '',
    enabled: true,
    assetCount: 0,
    profileCount: registration.effective.profiles.length,
    variantCount: registration.effective.profiles.reduce((total, profile) => total + profile.variants.length, 0),
  }]
}))
const profiles = computed<Profile[]>(() => registrationStates.value.flatMap(state =>
  state.registration?.effective.profiles.map(profile => ({
    siteKey: state.registration!.namespaceKey,
    profileKey: profile.key,
    purpose: profile.purpose,
    storageBackend: profile.storageBackend || '',
    allowedExt: profile.allowedMimes.join(','),
    maxSizeBytes: profile.maxBytes,
    defaultVisibility: profile.visibility,
    defaultDeliveryPolicy: profile.visibility === 'public' ? 'public' : 'signed',
    keepOriginal: profile.keepOriginal,
    metadataPolicy: profile.metadataPolicy,
    assetCount: 0,
    variantCount: profile.variants.length,
  })) || [],
))
const assets = computed(() => assetCollection.value.items)
const totalAssets = computed(() => assetCollection.value.total)
function maintenanceSelectedAssetIds() {
  return [...selectedIds.value]
}
function clearMaintenanceSelection() {
  clearSelection()
}
const {
  storageBackends,
  setStorageBackends,
  storageBackendOpen,
  savingStorageBackend,
  loadingStorageBackend,
  storageBackendEditingName,
  storageBackendDetail,
  storageBackendEvents,
  checkingStorageBackend,
  rotatingStorageBackend,
  rotateStorageBackendOpen,
  deleteStorageBackendOpen,
  deletingStorageBackend,
  storageBackendForm,
  storageBackendTypeItems,
  editStorageBackend,
  saveStorageBackend,
  confirmDeleteStorageBackend,
  fetchStorageBackendEvents,
  checkStorageBackendHealth,
  confirmRotateStorageBackendSecret,
} = useAssetStorageBackends({ reloadAll })
const {
  grants,
  totalGrants,
  grantPage,
  grantPageSize,
  fetchGrants,
  revokeGrantTarget,
  revokeGrantOpen,
  revokingGrant,
  requestRevokeGrant,
  confirmRevokeGrant,
  createGrantOpen,
  creatingGrant,
  grantAsset,
  createdGrant,
  grantForm,
  openCreateGrant,
  createGrant,
} = useAssetGrants({ reloadAll })
const {
  references,
  referenceAsset,
  referencesOpen,
  loadingReferences,
  openReferences,
  deleteAssetTarget,
  deleteAssetOpen,
  deletingAsset,
  requestDeleteAsset,
  confirmDeleteAsset,
  securityAsset,
  securityDetail,
  securityOpen,
  loadingSecurity,
  retryingSecurity,
  openSecurity,
  retrySecurity,
  deleteFromSecurity,
  securityRejectTarget,
  securityRejectOpen,
  rejectingSecurity,
  requestRejectSecurity,
  confirmRejectSecurity,
} = useAssetLibraryActions({ reloadAll })
const {
  maintenanceTasks,
  controllingMaintenanceTaskId,
  queueingSelectedRebuild,
  selectedRebuildError,
  selectedTask,
  activeMaintenanceCount,
  fetchMaintenanceTasks,
  showQueuedMaintenanceTask,
  controlMaintenanceTask,
  dismissSelectedTask,
  controlSelectedTask,
  queueSelectedRebuild,
} = useAssetMaintenanceTasks({
  selectedTaskId,
  getSelectedAssetIds: maintenanceSelectedAssetIds,
  clearSelection: clearMaintenanceSelection,
  refreshAfterSettled: refreshAfterMaintenanceTasksSettled,
})
const {
  operationFeedback,
  dismissOperationFeedback,
  rebuildingAssetId,
  sweepingStaging,
  pruneOpen,
  pruningUnreferenced,
  prunePreview,
  pruneForm,
  orphanObjectsOpen,
  auditingOrphanObjects,
  orphanObjectsPreview,
  orphanObjectsForm,
  storageMigrationOpen,
  migratingStorage,
  storageMigrationPreview,
  storageMigrationForm,
  batchRebuildOpen,
  batchRebuilding,
  batchRebuildProfile,
  batchRebuildPreview,
  batchRebuildLimit,
  rebuildDerivatives,
  previewBatchRebuild,
  refreshBatchRebuildPreview,
  confirmBatchRebuild,
  sweepStaging,
  previewPruneUnreferenced,
  confirmPruneUnreferenced,
  previewOrphanObjects,
  confirmPruneOrphanObjects,
  openStorageMigration,
  previewStorageMigration,
  confirmStorageMigration,
} = useAssetMaintenanceOperations({
  storageBackends,
  fetchMaintenanceTasks,
  showQueuedTask: showQueuedMaintenanceTask,
  reloadAll,
})
const currentSectionIssue = computed(() => sectionIssues[tab.value])
const currentSectionLoading = computed(() =>
  tab.value === 'library' ? assetPending.value : sectionLoading[tab.value],
)
const showSkeleton = useMinimumLoading(computed(() => {
  if (!mounted.value) return true
  if (tab.value === 'library') return false
  return sectionLoading[tab.value] && !sectionReady[tab.value]
}))
const enabledStorageBackends = computed(() => storageBackends.value.filter((b) => b.enabled !== false))
const sectionItems = computed<
  Array<{
    label: string
    icon: string
    value: AssetAdminSection
    badge: number | string
  }>
>(() => [
  {
    label: '素材库',
    icon: 'i-tabler-photo',
    value: 'library',
    badge: ['idle', 'loading'].includes(assetCollection.value.loadState) ? '—' : totalAssets.value,
  },
  {
    label: '消费者注册',
    icon: 'i-tabler-plug-connected',
    value: 'registrations',
    badge: sectionReady.registrations ? registrationStates.value.length : '—',
  },
  {
    label: '存储',
    icon: 'i-tabler-database',
    value: 'storage',
    badge: sectionReady.storage ? storageBackends.value.length : '—',
  },
  {
    label: '维护',
    icon: 'i-tabler-tool',
    value: 'maintenance',
    badge: sectionReady.maintenance ? activeMaintenanceCount.value || maintenanceTasks.value.length : '—',
  },
  {
    label: '授权',
    icon: 'i-tabler-key',
    value: 'grants',
    badge: sectionReady.grants ? totalGrants.value : '—',
  },
])

const siteOptions = computed(() => [
  { label: '全部站点', value: ALL },
  ...sites.value.map((s) => ({
    label: `${s.name} / ${s.siteKey}`,
    value: s.siteKey,
  })),
])
const profileOptions = computed(() => [
  { label: '全部 Profile', value: ALL },
  ...profiles.value
    .filter((p) => siteKey.value === ALL || p.siteKey === siteKey.value)
    .map((p) => ({
      label: `${p.profileKey} / ${p.siteKey}`,
      value: p.profileKey,
    })),
])
const visibilityOptions = [
  { label: '全部可见性', value: ALL },
  { label: '公开', value: 'public' },
  { label: '私有', value: 'private' },
]
const mimeOptions = [
  { label: '全部类型', value: ALL },
  { label: '图片', value: 'image/' },
  { label: '视频', value: 'video/' },
  { label: '音频', value: 'audio/' },
  { label: 'PDF', value: 'application/pdf' },
  { label: '其它文件', value: 'application/' },
]
const sortOptions = [
  { label: '按上传时间', value: 'createdAt' },
  { label: '按文件名', value: 'filename' },
  { label: '按文件大小', value: 'size' },
]
const pageSizeItems = [12, 24, 48, 96].map((value) => ({
  label: `${value}/页`,
  value,
}))
const activeLibraryFilterCount = computed(
  () => [spaceKey.value, siteKey.value, profileKey.value, visibility.value, mime.value].filter((value) => value !== ALL).length,
)
const storageBackendOptions = computed(() =>
  storageBackends.value
    .filter((b) => b.enabled !== false)
    .map((b) => ({
      label: b.isDefault ? `${b.name} · 默认` : b.name,
      value: b.name,
    })),
)
const spaceOptions = computed(() => [
  { label: '全部资源空间', value: ALL },
  ...spaces.value.map((space) => ({
    label: `${space.spaceKey} · ${space.assetCount} 个 / ${formatBytes(space.totalBytes)}`,
    value: space.spaceKey,
  })),
])
const publicPolicyOptions = [{ label: '公开直链', value: 'public' }]
const privatePolicyOptions = [
  { label: '短期签名', value: 'signed' },
  { label: '一次性', value: 'oneTime' },
  { label: '付费', value: 'paid' },
  { label: '门禁', value: 'gated' },
]
const policyOptions = [...publicPolicyOptions, ...privatePolicyOptions]

const selectedIds = computed<readonly string[]>(() => (assetCollection.value.selection.mode === 'keys' ? assetCollection.value.selection.keys : []))
const isPageSelected = computed(() => assetCollection.value.isPageSelected)
const isPageIndeterminate = computed(() => assetCollection.value.isPageIndeterminate)
function toggleOne(id: string) {
  assetWorkflow.toggleKey(id)
}
function togglePage(selected: boolean) {
  assetWorkflow.togglePage(selected)
}
function clearSelection() {
  assetWorkflow.clearSelection()
}
function submitAssetSearch(value: string) {
  if (searchTimer) clearTimeout(searchTimer)
  searchInput.value = value
  updateCollectionQuery({ q: value.trim() })
}

const assetPending = computed(
  () => assetCollection.value.loadState === 'loading' || assetCollection.value.loadState === 'idle',
)
const showLibrarySkeleton = useMinimumLoading(computed(() => !mounted.value || assetPending.value))
const libraryState = computed<CollectionPanelState>(() => {
  if (assetCollection.value.issue) return 'error'
  return showLibrarySkeleton.value ? 'loading' : 'ready'
})
const libraryErrorMessage = computed(() => (assetCollection.value.issue ? '暂时无法读取素材，请检查网络后重试。' : ''))

function openMaintenanceTaskList() {
  tab.value = 'maintenance'
}

onMounted(async () => {
  mounted.value = true
  await reloadAll()
})

async function loadSection(
  key: Exclude<AssetAdminSection, 'library'>,
  load: () => Promise<void>,
) {
  sectionLoading[key] = true
  sectionIssues[key] = ''
  try {
    await load()
    sectionReady[key] = true
  } catch (error) {
    sectionIssues[key] = apiErrorMessage(error, {
      fallback: '暂时无法加载当前资源数据，请检查服务状态后重试。',
    })
  } finally {
    sectionLoading[key] = false
  }
}

async function reloadSection(key: AssetAdminSection) {
  if (key === 'library') {
    sectionIssues.library = ''
    try {
      const spaceData = await call<{ items: AssetSpaceUsage[] }>('/api/v1/admin/assets-proxy/spaces')
      spaces.value = spaceData.items ?? []
      await reloadAssets()
    } catch (error) {
      sectionIssues.library = apiErrorMessage(error, {
        fallback: '暂时无法加载素材库，请检查服务状态后重试。',
      })
    }
    return
  }
  if (key === 'registrations') {
    await loadSection(key, async () => {
      const data = await call<{ items: AssetConsumerRegistrationState[] }>('/api/v1/admin/assets-proxy/registrations')
      registrationStates.value = data.items ?? []
    })
    return
  }
  if (key === 'storage') {
    await loadSection(key, async () => {
      const data = await call<{ items: StorageBackend[]; defaultName: string }>('/api/v1/admin/assets-proxy/storage-backends')
      setStorageBackends(data.items ?? [])
    })
    return
  }
  if (key === 'maintenance') {
    await loadSection(key, fetchMaintenanceTasks)
    return
  }
  await loadSection(key, fetchGrants)
}

async function reloadAll(includeAssets = true) {
  loading.value = true
  try {
    await Promise.all([
      ...(includeAssets ? [reloadSection('library')] : []),
      reloadSection('registrations'),
      reloadSection('storage'),
      reloadSection('maintenance'),
      reloadSection('grants'),
    ])
  } finally {
    loading.value = false
  }
}

async function refreshAfterMaintenanceTasksSettled() {
  await Promise.all([reloadSection('library'), reloadSection('registrations')])
}

function formatBytes(n: number) {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`
}
const previewVariantPreference = ['thumbnail', 'card', 'inline', 'home', 'content', 'og']
function assetPreviewURL(asset: AssetItem) {
  if (!asset.mediaKey || asset.visibility !== 'public' || !asset.mime.startsWith('image/')) return ''
  const registration = registrationStates.value.find((state) =>
    state.registration?.namespaceKey === asset.siteKey,
  )?.registration
  const variants = registration?.effective.profiles.find((profile) =>
    profile.key === asset.profileKey,
  )?.variants.filter((variant) => variant.visibility === 'public') || []
  const variant = [...variants].sort((left, right) => {
    const leftIndex = previewVariantPreference.indexOf(left.key)
    const rightIndex = previewVariantPreference.indexOf(right.key)
    return (leftIndex < 0 ? previewVariantPreference.length : leftIndex)
      - (rightIndex < 0 ? previewVariantPreference.length : rightIndex)
  })[0]
  if (!variant) return ''
  const format = variant.format.toLowerCase() === 'jpeg' ? 'jpg' : variant.format.toLowerCase()
  return `/media/${asset.mediaKey}?format=${encodeURIComponent(format)}&name=${encodeURIComponent(variant.key)}`
}
function assetActions(asset: AssetItem): DropdownMenuItem[][] {
  return [
    [
      {
        label: '安全详情',
        icon: 'i-tabler-shield-search',
        onSelect: () => openSecurity(asset),
      },
      {
        label: '查看引用',
        icon: 'i-tabler-link',
        disabled: !asset.refCount,
        onSelect: () => openReferences(asset),
      },
      {
        label: '打开文件',
        icon: 'i-tabler-external-link',
        disabled: asset.securityState !== 'ready' || !assetPreviewURL(asset),
        onSelect: () => {
          const url = assetPreviewURL(asset)
          if (url) window.open(url, '_blank')
        },
      },
      {
        label: '签发交付链接',
        icon: 'i-tabler-key',
        disabled: asset.securityState !== 'ready',
        onSelect: () => openCreateGrant(asset),
      },
      {
        label: '重建派生图',
        icon: 'i-tabler-refresh-dot',
        disabled: asset.securityState !== 'ready' || !asset.mime.startsWith('image/') || rebuildingAssetId.value === asset.id,
        onSelect: () => rebuildDerivatives(asset),
      },
    ],
    [
      {
        label: asset.refCount ? '有引用，不能删除' : '删除素材',
        icon: 'i-tabler-trash',
        color: 'error',
        disabled: !!asset.refCount,
        onSelect: () => requestDeleteAsset(asset),
      },
    ],
  ]
}
function grantStatus(grant: Grant): {
  label: string
  color: 'success' | 'warning' | 'error' | 'neutral'
} {
  if (grant.revokedAt) return { label: '已撤销', color: 'neutral' }
  if (grant.expiresAt && new Date(grant.expiresAt).getTime() <= Date.now()) return { label: '已过期', color: 'warning' }
  if (grant.maxUses > 0 && grant.usedCount >= grant.maxUses) return { label: '已用完', color: 'warning' }
  return { label: '有效', color: 'success' }
}
function canRevokeGrant(grant: Grant) {
  return grantStatus(grant).label === '有效'
}
function grantActions(grant: Grant): DropdownMenuItem[][] {
  return [
    [
      {
        label: '撤销授权',
        icon: 'i-tabler-ban',
        color: 'error',
        disabled: !canRevokeGrant(grant),
        onSelect: () => requestRevokeGrant(grant),
      },
    ],
  ]
}
</script>

<template>
  <div class="w-full space-y-5">
    <PageHeader title="资源管理" icon="i-tabler-photo-cog">
      <template #actions>
        <div class="flex items-center gap-2">
          <UTooltip text="刷新资源数据">
            <UButton
              icon="i-tabler-refresh"
              color="neutral"
              variant="ghost"
              square
              :loading="loading"
              aria-label="刷新资源数据"
              @click="reloadAll()"
            />
          </UTooltip>
          <UButton v-if="tab === 'storage'" icon="i-tabler-database-plus" label="新建后端" @click="editStorageBackend()" />
          <UButton
            v-else-if="tab === 'maintenance'"
            icon="i-tabler-transfer"
            label="迁移后端"
            :disabled="enabledStorageBackends.length < 2"
            @click="openStorageMigration"
          />
        </div>
      </template>
    </PageHeader>

    <TabbedSurface
      v-model="tab"
      :items="sectionItems"
      navigation-label="资源管理分区"
      data-asset-control-surface
    >
      <div v-if="operationFeedback" class="border-b border-default px-4 py-3 sm:px-5">
        <UAlert
          :color="operationFeedback.tone"
          variant="subtle"
          :icon="operationFeedback.tone === 'warning' ? 'i-tabler-alert-triangle' : 'i-tabler-circle-check'"
          :title="operationFeedback.title"
          :description="operationFeedback.description"
          role="status"
        >
          <template #actions>
            <UButton label="关闭" color="neutral" variant="ghost" size="xs" @click="dismissOperationFeedback" />
          </template>
        </UAlert>
      </div>

      <div v-if="currentSectionIssue" class="border-b border-default px-4 py-3 sm:px-5">
        <UAlert
          color="error"
          variant="subtle"
          icon="i-tabler-alert-circle"
          title="资源数据加载失败"
          :description="currentSectionIssue"
        >
          <template #actions>
            <UButton
              label="重新加载"
              color="neutral"
              variant="outline"
              size="xs"
              :loading="currentSectionLoading"
              @click="reloadSection(tab)"
            />
          </template>
        </UAlert>
      </div>

      <LazyAssetLibraryPanel
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
        class="p-3 sm:p-5"
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
        :sites="sites"
        :state="libraryState"
        :error-message="libraryErrorMessage"
        :queueing="queueingSelectedRebuild"
        :queue-error="selectedRebuildError"
        :task-id="selectedTaskId"
        :task="selectedTask"
        :controlling-task-id="controllingMaintenanceTaskId"
        :actions-for="assetActions"
        :preview-for="assetPreviewURL"
        @search="submitAssetSearch"
        @retry="reloadAssets"
        @toggle-one="toggleOne"
        @toggle-page="togglePage"
        @queue-selected="queueSelectedRebuild"
        @clear-selection="clearSelection"
        @task-action="controlSelectedTask"
        @open-maintenance="openMaintenanceTaskList"
        @dismiss-task="dismissSelectedTask"
      />

      <div v-else-if="showSkeleton" class="p-3 sm:p-5">
        <SkeletonList :rows="7" />
      </div>

      <template v-else>
        <LazyAssetStoragePanel
          v-if="tab === 'storage'"
          class="p-4 sm:p-5"
          :backends="storageBackends"
          @edit="editStorageBackend"
        />

        <LazyAssetMaintenancePanel
          v-else-if="tab === 'maintenance'"
          class="p-4 sm:p-5"
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

        <LazyAssetRegistrationsPanel
          v-else-if="tab === 'registrations'"
          class="p-4 sm:p-5"
          :states="registrationStates"
        />

        <LazyAssetGrantsPanel
          v-else-if="tab === 'grants'"
          v-model:page="grantPage"
          class="p-4 sm:p-5"
          :grants="grants"
          :total="totalGrants"
          :page-size="grantPageSize"
          :actions-for="grantActions"
        />
      </template>
    </TabbedSurface>

    <LazyAssetStorageBackendModal
      v-if="storageBackendOpen"
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

    <LazyAssetStorageBackendDeleteModal
      v-if="deleteStorageBackendOpen"
      v-model:open="deleteStorageBackendOpen"
      :backend-name="storageBackendEditingName"
      :deleting="deletingStorageBackend"
      @confirm="confirmDeleteStorageBackend"
    />

    <LazyAssetStorageBackendRotateModal
      v-if="rotateStorageBackendOpen"
      v-model:open="rotateStorageBackendOpen"
      :backend-name="storageBackendEditingName"
      :rotating="rotatingStorageBackend"
      @confirm="confirmRotateStorageBackendSecret"
    />

    <LazyAssetDeleteConfirmModal
      v-if="deleteAssetOpen"
      v-model:open="deleteAssetOpen"
      title="删除素材？"
      description="原文件、派生图和相关交付授权会一并失效。"
      :subject="deleteAssetTarget?.filename || deleteAssetTarget?.id"
      :deleting="deletingAsset"
      @confirm="confirmDeleteAsset"
    />

    <LazyAssetPruneModal
      v-if="pruneOpen"
      v-model:open="pruneOpen"
      :initial-value="pruneForm"
      :preview="prunePreview"
      :running="pruningUnreferenced"
      @preview="previewPruneUnreferenced"
      @confirm="confirmPruneUnreferenced"
    />

    <LazyAssetOrphanObjectsModal
      v-if="orphanObjectsOpen"
      v-model:open="orphanObjectsOpen"
      :initial-value="orphanObjectsForm"
      :preview="orphanObjectsPreview"
      :backend-options="[{ label: '全部后端', value: ALL }, ...storageBackendOptions]"
      :running="auditingOrphanObjects"
      @preview="previewOrphanObjects"
      @confirm="confirmPruneOrphanObjects"
    />

    <LazyAssetStorageMigrationModal
      v-if="storageMigrationOpen"
      v-model:open="storageMigrationOpen"
      :initial-value="storageMigrationForm"
      :preview="storageMigrationPreview"
      :backend-options="storageBackendOptions"
      :running="migratingStorage"
      @preview="previewStorageMigration"
      @confirm="confirmStorageMigration"
    />

    <LazyAssetReferencesModal
      v-if="referencesOpen"
      v-model:open="referencesOpen"
      :asset-id="referenceAsset?.id"
      :references="references"
      :loading="loadingReferences"
    />

    <LazyAssetSecurityModal
      v-if="securityOpen"
      v-model:open="securityOpen"
      :asset="securityAsset"
      :detail="securityDetail"
      :loading="loadingSecurity"
      :retrying="retryingSecurity"
      @retry="retrySecurity"
      @reject="requestRejectSecurity"
      @delete="deleteFromSecurity"
    />

    <LazyAssetSecurityRejectModal
      v-if="securityRejectOpen"
      v-model:open="securityRejectOpen"
      :asset="securityRejectTarget"
      :rejecting="rejectingSecurity"
      @confirm="confirmRejectSecurity"
    />

    <LazyAssetBatchRebuildModal
      v-if="batchRebuildOpen"
      v-model:open="batchRebuildOpen"
      :profile="batchRebuildProfile"
      :preview="batchRebuildPreview"
      :preview-limit="batchRebuildLimit"
      :running="batchRebuilding"
      @preview="refreshBatchRebuildPreview"
      @confirm="confirmBatchRebuild"
    />

    <LazyAssetGrantModal
      v-if="createGrantOpen"
      v-model:open="createGrantOpen"
      :asset-id="grantAsset?.id"
      :initial-value="grantForm"
      :created-grant="createdGrant"
      :policy-options="policyOptions"
      :creating="creatingGrant"
      @create="createGrant"
    />

    <LazyAssetRevokeGrantModal
      v-if="revokeGrantOpen"
      v-model:open="revokeGrantOpen"
      :asset-id="revokeGrantTarget?.assetId"
      :revoking="revokingGrant"
      @confirm="confirmRevokeGrant"
    />
  </div>
</template>
