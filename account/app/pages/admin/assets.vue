<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import { SkeletonList } from '~/components/manage'
import { createCollectionRouteQueryCodec, createJsonCollectionQueryPolicy, type CollectionPanelState, type CollectionWorkflow } from '@yueli/ui/collection'
import { useVueCollectionWorkflow } from '@yueli/ui/collection/vue'
import { createVueRouterCollectionQuerySync } from '@yueli/ui/collection/vue-router'
import { PageHeader } from '@yueli/ui/dashboard/pattern'
import { useMinimumLoading } from '@yueli/ui/feedback'
import { createAccountNotifier } from '~/utils/feedback'
import type {
  AssetAdminSection,
  AssetAdminStats as Stats,
  AssetGrant as Grant,
  AssetItem,
  AssetProfile as Profile,
  AssetSite as Site,
  AssetSpaceUsage,
  AssetStorageBackend as StorageBackend,
  AssetVariant as Variant,
} from '~/types/asset-admin'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '资源管理 · 控制台' })

const { call } = useApi()
const toast = createAccountNotifier(useToast())
const router = useRouter()
const ALL = '__all__' as const

const mounted = ref(false)
const loading = ref(true)
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
const sections = ['library', 'sites', 'profiles', 'storage', 'maintenance', 'grants'] as const
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
const stats = ref<Stats>({
  assets: 0,
  publicAssets: 0,
  privateAssets: 0,
  sites: 0,
  profiles: 0,
  activeGrants: 0,
})
const spaces = ref<AssetSpaceUsage[]>([])
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
  sites,
  profiles,
  variants,
  setConfigurationData,
  profileOpen,
  savingProfile,
  profileForm,
  editProfile,
  saveProfile,
  siteOpen,
  savingSite,
  siteForm,
  editSite,
  saveSite,
  variantOpen,
  savingVariant,
  variantForm,
  editVariant,
  saveVariant,
  deleteVariantTarget,
  deleteVariantOpen,
  deletingVariant,
  requestDeleteVariant,
  confirmDeleteVariant,
  deleteProfileTarget,
  deleteProfileOpen,
  deletingProfile,
  requestDeleteProfile,
  confirmDeleteProfile,
} = useAssetConfiguration({
  storageBackends,
  currentSiteKey: siteKey,
  allSiteKey: ALL,
  reloadAll,
})
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
const showSkeleton = useMinimumLoading(computed(() => !mounted.value || loading.value))
const enabledStorageBackends = computed(() => storageBackends.value.filter((b) => b.enabled !== false))
const sectionItems = computed<
  Array<{
    label: string
    icon: string
    value: AssetAdminSection
    count: number
  }>
>(() => [
  {
    label: '素材库',
    icon: 'i-tabler-photo',
    value: 'library',
    count: stats.value.assets,
  },
  {
    label: '站点',
    icon: 'i-tabler-world',
    value: 'sites',
    count: stats.value.sites,
  },
  {
    label: 'Profile',
    icon: 'i-tabler-folder-cog',
    value: 'profiles',
    count: stats.value.profiles,
  },
  {
    label: '存储',
    icon: 'i-tabler-database',
    value: 'storage',
    count: storageBackends.value.length,
  },
  {
    label: '维护',
    icon: 'i-tabler-tool',
    value: 'maintenance',
    count: activeMaintenanceCount.value || maintenanceTasks.value.length,
  },
  {
    label: '授权',
    icon: 'i-tabler-key',
    value: 'grants',
    count: stats.value.activeGrants,
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
const modeOptions = [
  { label: '等比缩放', value: 'resize' },
  { label: '填充裁剪', value: 'fill' },
]
const publicPolicyOptions = [{ label: '公开直链', value: 'public' }]
const privatePolicyOptions = [
  { label: '短期签名', value: 'signed' },
  { label: '一次性', value: 'oneTime' },
  { label: '付费', value: 'paid' },
  { label: '门禁', value: 'gated' },
]
const policyOptions = [...publicPolicyOptions, ...privatePolicyOptions]
const profileAccessLevelOptions = [
  { label: '公开资源 · 公开直链', value: 'public' },
  { label: '私有资源 · 签名链接', value: 'private' },
]

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
  () => assetCollection.value.loadState === 'loading' || assetCollection.value.loadState === 'refreshing' || assetCollection.value.loadState === 'idle',
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
  await reloadAll(false)
})

async function reloadAll(includeAssets = true) {
  loading.value = true
  try {
    const [st, spaceData, siteData, backendData, profileData, variantData] = await Promise.all([
      call<Stats>('/api/v1/admin/assets-proxy/stats'),
      call<{ items: AssetSpaceUsage[] }>('/api/v1/admin/assets-proxy/spaces'),
      call<{ items: Site[] }>('/api/v1/admin/assets-proxy/sites'),
      call<{ items: StorageBackend[]; defaultName: string }>('/api/v1/admin/assets-proxy/storage-backends'),
      call<{ items: Profile[] }>('/api/v1/admin/assets-proxy/profiles'),
      call<{ items: Variant[] }>('/api/v1/admin/assets-proxy/variants'),
    ])
    stats.value = st
    spaces.value = spaceData.items ?? []
    setStorageBackends(backendData.items ?? [])
    setConfigurationData({
      sites: siteData.items ?? [],
      profiles: profileData.items ?? [],
      variants: variantData.items ?? [],
    })
    await Promise.all([...(includeAssets ? [reloadAssets()] : []), fetchGrants(), fetchMaintenanceTasks()])
  } catch (e) {
    toast.add({
      title: '资源后台加载失败',
      description: apiErrorMessage(e, { fallback: '暂时无法加载资源后台。' }),
      color: 'error',
    })
  } finally {
    loading.value = false
  }
}

async function refreshAfterMaintenanceTasksSettled() {
  const [st, siteData, profileData, variantData] = await Promise.all([
    call<Stats>('/api/v1/admin/assets-proxy/stats'),
    call<{ items: Site[] }>('/api/v1/admin/assets-proxy/sites'),
    call<{ items: Profile[] }>('/api/v1/admin/assets-proxy/profiles'),
    call<{ items: Variant[] }>('/api/v1/admin/assets-proxy/variants'),
  ])
  stats.value = st
  setConfigurationData({
    sites: siteData.items ?? [],
    profiles: profileData.items ?? [],
    variants: variantData.items ?? [],
  })
  await reloadAssets()
}

function formatBytes(n: number) {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(1)} GB`
}
function profileActions(profile: Profile): DropdownMenuItem[][] {
  const inUse = profile.assetCount > 0 || profile.variantCount > 0
  return [
    [
      {
        label: '编辑',
        icon: 'i-tabler-pencil',
        onSelect: () => editProfile(profile),
      },
      {
        label: '批量重建派生图',
        icon: 'i-tabler-refresh-dot',
        disabled: !profile.assetCount || batchRebuilding.value,
        onSelect: () => previewBatchRebuild(profile),
      },
      {
        label: '删除',
        icon: 'i-tabler-trash',
        color: 'error',
        disabled: inUse,
        onSelect: () => requestDeleteProfile(profile),
      },
    ],
  ]
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
        disabled: asset.securityState !== 'ready' || !asset.cdnUrl,
        onSelect: () => {
          if (asset.cdnUrl) window.open(asset.cdnUrl, '_blank')
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
function variantActions(variant: Variant): DropdownMenuItem[][] {
  return [
    [
      {
        label: '编辑',
        icon: 'i-tabler-pencil',
        onSelect: () => editVariant(variant),
      },
      {
        label: '删除',
        icon: 'i-tabler-trash',
        color: 'error',
        onSelect: () => requestDeleteVariant(variant),
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
  <div>
    <PageHeader title="资源管理">
      <template #subtitle>共享素材控制面：素材运营、安全授权、配置与存储维护</template>
      <template #actions>
        <div class="flex items-center gap-2">
          <UButton icon="i-tabler-refresh" label="刷新" color="neutral" variant="soft" @click="reloadAll()" />
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
    </PageHeader>

    <div class="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
      <div
        v-for="item in [
          ['资源', stats.assets],
          ['公开', stats.publicAssets],
          ['私有', stats.privateAssets],
          ['站点', stats.sites],
          ['Profile', stats.profiles],
          ['有效授权', stats.activeGrants],
        ]"
        :key="item[0]"
        class="rounded-lg border border-default bg-default px-4 py-3"
      >
        <div class="text-xs text-muted">{{ item[0] }}</div>
        <div class="mt-1 text-xl font-semibold text-highlighted tabular-nums">
          {{ item[1] }}
        </div>
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
        @click="
          () => {
            tab = item.value
          }
        "
      />
    </div>

    <UAlert
      v-if="operationFeedback"
      class="mb-5"
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
      :sites="sites"
      :state="libraryState"
      :error-message="libraryErrorMessage"
      :queueing="queueingSelectedRebuild"
      :queue-error="selectedRebuildError"
      :task-id="selectedTaskId"
      :task="selectedTask"
      :controlling-task-id="controllingMaintenanceTaskId"
      :actions-for="assetActions"
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

    <SkeletonList v-else-if="showSkeleton" :rows="8" />

    <template v-else>
      <AssetStoragePanel v-if="tab === 'storage'" :backends="storageBackends" @edit="editStorageBackend" />

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
        @add-variant="(profile) => editVariant(undefined, profile)"
      />

      <AssetGrantsPanel
        v-else-if="tab === 'grants'"
        v-model:page="grantPage"
        :grants="grants"
        :total="totalGrants"
        :page-size="grantPageSize"
        :actions-for="grantActions"
      />
    </template>

    <AssetProfileModal
      v-model:open="profileOpen"
      :initial-value="profileForm"
      :sites="sites"
      :site-options="siteOptions.filter((item) => item.value !== ALL)"
      :storage-backend-options="storageBackendOptions"
      :access-level-options="profileAccessLevelOptions"
      :saving="savingProfile"
      @save="saveProfile"
    />

    <AssetSiteModal v-model:open="siteOpen" :initial-value="siteForm" :storage-backend-options="storageBackendOptions" :saving="savingSite" @save="saveSite" />

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

    <AssetVariantModal v-model:open="variantOpen" :initial-value="variantForm" :mode-options="modeOptions" :saving="savingVariant" @save="saveVariant" />

    <AssetDeleteConfirmModal
      v-model:open="deleteProfileOpen"
      title="删除 Profile？"
      description="只有没有素材、没有 Variant 的 Profile 才允许删除。"
      :subject="deleteProfileTarget?.profileKey"
      :deleting="deletingProfile"
      @confirm="confirmDeleteProfile"
    />

    <AssetDeleteConfirmModal
      v-model:open="deleteAssetOpen"
      title="删除素材？"
      description="原文件、派生图和相关交付授权会一并失效。"
      :subject="deleteAssetTarget?.filename || deleteAssetTarget?.id"
      :deleting="deletingAsset"
      @confirm="confirmDeleteAsset"
    />

    <AssetPruneModal
      v-model:open="pruneOpen"
      :initial-value="pruneForm"
      :preview="prunePreview"
      :running="pruningUnreferenced"
      @preview="previewPruneUnreferenced"
      @confirm="confirmPruneUnreferenced"
    />

    <AssetOrphanObjectsModal
      v-model:open="orphanObjectsOpen"
      :initial-value="orphanObjectsForm"
      :preview="orphanObjectsPreview"
      :backend-options="[{ label: '全部后端', value: ALL }, ...storageBackendOptions]"
      :running="auditingOrphanObjects"
      @preview="previewOrphanObjects"
      @confirm="confirmPruneOrphanObjects"
    />

    <AssetStorageMigrationModal
      v-model:open="storageMigrationOpen"
      :initial-value="storageMigrationForm"
      :preview="storageMigrationPreview"
      :backend-options="storageBackendOptions"
      :running="migratingStorage"
      @preview="previewStorageMigration"
      @confirm="confirmStorageMigration"
    />

    <AssetReferencesModal v-model:open="referencesOpen" :asset-id="referenceAsset?.id" :references="references" :loading="loadingReferences" />

    <AssetSecurityModal
      v-model:open="securityOpen"
      :asset="securityAsset"
      :detail="securityDetail"
      :loading="loadingSecurity"
      :retrying="retryingSecurity"
      @retry="retrySecurity"
      @reject="requestRejectSecurity"
      @delete="deleteFromSecurity"
    />

    <AssetSecurityRejectModal
      v-model:open="securityRejectOpen"
      :asset="securityRejectTarget"
      :rejecting="rejectingSecurity"
      @confirm="confirmRejectSecurity"
    />

    <AssetDeleteConfirmModal
      v-model:open="deleteVariantOpen"
      title="删除 Variant？"
      description="已有文件不会被删除，但之后不会再生成这个派生规格。"
      :subject="deleteVariantTarget?.variantKey"
      :deleting="deletingVariant"
      @confirm="confirmDeleteVariant"
    />

    <AssetBatchRebuildModal
      v-model:open="batchRebuildOpen"
      :profile="batchRebuildProfile"
      :preview="batchRebuildPreview"
      :preview-limit="batchRebuildLimit"
      :running="batchRebuilding"
      @preview="refreshBatchRebuildPreview"
      @confirm="confirmBatchRebuild"
    />

    <AssetGrantModal
      v-model:open="createGrantOpen"
      :asset-id="grantAsset?.id"
      :initial-value="grantForm"
      :created-grant="createdGrant"
      :policy-options="policyOptions"
      :creating="creatingGrant"
      @create="createGrant"
    />

    <AssetRevokeGrantModal v-model:open="revokeGrantOpen" :asset-id="revokeGrantTarget?.assetId" :revoking="revokingGrant" @confirm="confirmRevokeGrant" />
  </div>
</template>
