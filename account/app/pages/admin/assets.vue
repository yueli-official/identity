<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import { ManageHeader, ManageEmpty, SkeletonList } from '@platform/ui/components'
import { useMinLoading } from '@platform/ui/use-min-loading'

definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '资源管理 · 控制台' })

const { call } = useApi()
const toast = useToast()
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
  isDefault: boolean
  healthy: boolean
  error?: string
}
interface Profile {
  siteKey: string
  profileKey: string
  purpose: string
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
  siteKey: string
  profileKey: string
  deliveryPolicy: string
  storageBackend: string
  refCount: number
  cdnUrl?: string
  createdAt: string
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
interface Grant {
  id: string
  assetId: string
  variantKey: string
  siteKey: string
  subjectId: string
  policy: string
  expiresAt: string
  maxUses: number
  usedCount: number
  revokedAt?: string
  createdByService: string
  reason: string
  createdAt: string
}

const mounted = ref(false)
const loading = ref(true)
const tab = ref<'library' | 'profiles' | 'grants'>('library')
const stats = ref<Stats>({ assets: 0, publicAssets: 0, privateAssets: 0, sites: 0, profiles: 0, activeGrants: 0 })
const sites = ref<Site[]>([])
const storageBackends = ref<StorageBackend[]>([])
const profiles = ref<Profile[]>([])
const variants = ref<Variant[]>([])
const assets = ref<AssetItem[]>([])
const references = ref<AssetReference[]>([])
const grants = ref<Grant[]>([])
const totalAssets = ref(0)
const totalGrants = ref(0)
const page = ref(1)
const grantPage = ref(1)
const SIZE = 20

const siteKey = ref(ALL)
const profileKey = ref(ALL)
const visibility = ref(ALL)
const showSkeleton = useMinLoading(computed(() => !mounted.value || loading.value))

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
const storageBackendOptions = computed(() => storageBackends.value.map(b => ({
  label: b.isDefault ? `${b.name} · 默认` : b.name,
  value: b.name
})))
const modeOptions = [
  { label: '等比缩放', value: 'resize' },
  { label: '填充裁剪', value: 'fill' }
]
const policyOptions = [
  { label: '公开', value: 'public' },
  { label: '短期签名', value: 'signed' },
  { label: '一次性', value: 'oneTime' },
  { label: '付费', value: 'paid' },
  { label: '门禁', value: 'gated' }
]

onMounted(async () => {
  mounted.value = true
  await reloadAll()
})

watch([siteKey, profileKey, visibility], async () => {
  page.value = 1
  await fetchAssets()
})
watch(page, fetchAssets)
watch(grantPage, fetchGrants)

async function reloadAll() {
  loading.value = true
  try {
    const [st, siteData, backendData, profileData, variantData] = await Promise.all([
      call<Stats>('/api/v1/admin/assets-proxy/stats'),
      call<{ items: Site[] }>('/api/v1/admin/assets-proxy/sites'),
      call<{ items: StorageBackend[], defaultName: string }>('/api/v1/admin/assets-proxy/storage-backends'),
      call<{ items: Profile[] }>('/api/v1/admin/assets-proxy/profiles'),
      call<{ items: Variant[] }>('/api/v1/admin/assets-proxy/variants')
    ])
    stats.value = st
    sites.value = siteData.items ?? []
    storageBackends.value = backendData.items ?? []
    profiles.value = profileData.items ?? []
    variants.value = variantData.items ?? []
    await Promise.all([fetchAssets(), fetchGrants()])
  } catch (e) {
    toast.add({ title: '资源后台加载失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    loading.value = false
  }
}

async function fetchAssets() {
  const data = await call<{ items: AssetItem[], total: number }>('/api/v1/admin/assets-proxy/library', {
    params: {
      page: page.value,
      size: SIZE,
      siteKey: siteKey.value !== ALL ? siteKey.value : undefined,
      profileKey: profileKey.value !== ALL ? profileKey.value : undefined,
      visibility: visibility.value !== ALL ? visibility.value : undefined
    }
  })
  assets.value = data.items ?? []
  totalAssets.value = data.total ?? 0
}

async function fetchGrants() {
  const data = await call<{ items: Grant[], total: number }>('/api/v1/admin/assets-proxy/grants', {
    params: { page: grantPage.value, size: SIZE }
  })
  grants.value = data.items ?? []
  totalGrants.value = data.total ?? 0
}

const profileOpen = ref(false)
const profileForm = reactive<Profile>({
  siteKey: 'platform',
  profileKey: '',
  purpose: '',
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
  await call('/api/v1/admin/assets-proxy/profiles', { method: 'POST', body: profileForm })
  toast.add({ title: 'Profile 已保存', color: 'success', icon: 'i-tabler-check' })
  profileOpen.value = false
  await reloadAll()
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
    { label: '删除', icon: 'i-tabler-trash', color: 'error', disabled: inUse, onSelect: () => { deleteProfileTarget.value = profile } }
  ]]
}
function assetActions(asset: AssetItem): DropdownMenuItem[][] {
  return [[
    { label: '查看引用', icon: 'i-tabler-link', disabled: !asset.refCount, onSelect: () => openReferences(asset) },
    { label: '打开文件', icon: 'i-tabler-external-link', disabled: !asset.cdnUrl, onSelect: () => { if (asset.cdnUrl) window.open(asset.cdnUrl, '_blank') } }
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
          <UButton icon="i-tabler-world-plus" label="新建站点" color="neutral" variant="soft" @click="editSite()" />
          <UButton icon="i-tabler-plus" label="新建 Profile" @click="editProfile()" />
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
      <UButton label="素材库" icon="i-tabler-photo" :variant="tab === 'library' ? 'solid' : 'ghost'" @click="tab = 'library'" />
      <UButton label="站点配置" icon="i-tabler-adjustments" color="neutral" :variant="tab === 'profiles' ? 'soft' : 'ghost'" @click="tab = 'profiles'" />
      <UButton label="交付授权" icon="i-tabler-key" color="neutral" :variant="tab === 'grants' ? 'soft' : 'ghost'" @click="tab = 'grants'" />
    </div>

    <SkeletonList v-if="showSkeleton" :rows="8" />

    <template v-else>
      <section v-if="tab === 'library'" class="space-y-4">
        <div class="flex flex-wrap items-center gap-3 rounded-lg border border-default bg-default p-3">
          <USelectMenu v-model="siteKey" :items="siteOptions" value-key="value" class="w-56" :search-input="{ placeholder: '搜索站点…' }" />
          <USelectMenu v-model="profileKey" :items="profileOptions" value-key="value" class="w-56" :search-input="{ placeholder: '搜索 Profile…' }" />
          <USelect v-model="visibility" :items="visibilityOptions" value-key="value" class="w-36" />
        </div>

        <ManageEmpty v-if="!assets.length" icon="i-tabler-photo-off" text="没有匹配的资源" />
        <div v-else class="overflow-hidden rounded-lg border border-default bg-default">
          <div
            v-for="asset in assets"
            :key="asset.id"
            class="flex items-center gap-4 border-b border-default px-4 py-3 last:border-b-0"
          >
            <div class="grid size-12 shrink-0 place-items-center overflow-hidden rounded-lg bg-elevated">
              <img v-if="asset.cdnUrl && asset.mime.startsWith('image/')" :src="asset.cdnUrl" :alt="asset.filename" class="size-full object-cover">
              <UIcon v-else name="i-tabler-file" class="size-5 text-muted" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-highlighted">{{ asset.filename || asset.id }}</div>
              <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-muted">
                <span>{{ siteName(asset.siteKey) }}</span>
                <span>/ {{ asset.profileKey }}</span>
                <span>{{ asset.mime }}</span>
                <span>{{ formatBytes(asset.size) }}</span>
                <span>{{ asset.storageBackend || 'local' }}</span>
                <span v-if="asset.width && asset.height">{{ asset.width }}x{{ asset.height }}</span>
              </div>
            </div>
            <div class="hidden items-center gap-2 sm:flex">
              <UBadge v-if="asset.refCount" :label="`${asset.refCount} 引用`" color="warning" variant="soft" />
              <UBadge :label="asset.visibility" :color="asset.visibility === 'public' ? 'success' : 'warning'" variant="soft" />
              <UBadge :label="asset.deliveryPolicy || 'public'" color="neutral" variant="soft" />
            </div>
            <UDropdownMenu :items="assetActions(asset)">
              <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="xs" />
            </UDropdownMenu>
          </div>
        </div>
        <div class="flex items-center justify-between text-sm text-muted">
          <span>共 {{ totalAssets }} 个资源</span>
          <div class="flex items-center gap-2">
            <UButton icon="i-tabler-chevron-left" color="neutral" variant="ghost" :disabled="page <= 1" @click="page--" />
            <span>{{ page }}</span>
            <UButton icon="i-tabler-chevron-right" color="neutral" variant="ghost" :disabled="page * SIZE >= totalAssets" @click="page++" />
          </div>
        </div>
      </section>

      <section v-else-if="tab === 'profiles'" class="space-y-4">
        <div class="rounded-lg border border-default bg-default p-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-highlighted">存储后端</h2>
              <p class="mt-1 text-xs text-muted">站点上传会落到自己的默认后端；不存在的后端不能保存。</p>
            </div>
            <UBadge :label="`${storageBackends.length} 个可用`" color="neutral" variant="soft" />
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <UBadge
              v-for="backend in storageBackends"
              :key="backend.name"
              :label="`${backend.name}${backend.isDefault ? ' · 默认' : ''}${backend.healthy ? '' : ' · 异常'}`"
              :color="backend.healthy ? (backend.isDefault ? 'primary' : 'neutral') : 'error'"
              variant="soft"
            />
          </div>
        </div>

        <div class="grid gap-3 lg:grid-cols-2">
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

        <div class="flex items-center justify-between pt-2">
          <h2 class="text-sm font-semibold text-highlighted">Profile 与派生规格</h2>
          <UButton icon="i-tabler-plus" label="新建 Profile" size="sm" @click="editProfile()" />
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
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
                <div>可见性 <span class="text-default">{{ profile.defaultVisibility }}</span></div>
                <div>交付 <span class="text-default">{{ profile.defaultDeliveryPolicy }}</span></div>
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

      <section v-else class="space-y-4">
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
            </div>
            <UBadge :label="grantStatus(grant).label" :color="grantStatus(grant).color" variant="soft" />
            <UBadge :label="grant.policy" color="neutral" variant="soft" />
            <UDropdownMenu :items="grantActions(grant)">
              <UButton icon="i-tabler-dots-vertical" color="neutral" variant="ghost" square size="xs" />
            </UDropdownMenu>
          </div>
        </div>
        <div class="flex items-center justify-between text-sm text-muted">
          <span>共 {{ totalGrants }} 条授权</span>
          <div class="flex items-center gap-2">
            <UButton icon="i-tabler-chevron-left" color="neutral" variant="ghost" :disabled="grantPage <= 1" @click="grantPage--" />
            <span>{{ grantPage }}</span>
            <UButton icon="i-tabler-chevron-right" color="neutral" variant="ghost" :disabled="grantPage * SIZE >= totalGrants" @click="grantPage++" />
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
          <UFormField label="允许后缀"><UInput v-model="profileForm.allowedExt" placeholder="jpg,jpeg,png,webp" /></UFormField>
          <UFormField label="大小上限(bytes)"><UInput v-model.number="profileForm.maxSizeBytes" type="number" /></UFormField>
          <UFormField label="默认可见性"><USelect v-model="profileForm.defaultVisibility" :items="[{ label: '公开', value: 'public' }, { label: '私有', value: 'private' }]" value-key="value" /></UFormField>
          <UFormField label="默认交付"><USelect v-model="profileForm.defaultDeliveryPolicy" :items="policyOptions" value-key="value" /></UFormField>
          <UCheckbox v-model="profileForm.keepOriginal" label="保留原图/原文件" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton label="取消" color="neutral" variant="ghost" @click="profileOpen = false" />
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
          <UButton label="取消" color="neutral" variant="ghost" @click="siteOpen = false" />
          <UButton label="保存" icon="i-tabler-device-floppy" @click="saveSite" />
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
          <UButton label="取消" color="neutral" variant="ghost" @click="variantOpen = false" />
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingProfile" @click="deleteProfileTarget = null" />
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingAsset" @click="deleteAssetTarget = null" />
          <UButton color="error" label="确认删除" :loading="deletingAsset" @click="confirmDeleteAsset" />
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deletingVariant" @click="deleteVariantTarget = null" />
          <UButton color="error" label="确认删除" :loading="deletingVariant" @click="confirmDeleteVariant" />
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="revokingGrant" @click="revokeGrantTarget = null" />
          <UButton color="error" label="确认撤销" :loading="revokingGrant" @click="confirmRevokeGrant" />
        </div>
      </template>
    </UModal>
  </div>
</template>
