<script setup lang="ts">
import {
  createCollectionRouteQueryCodec,
  createJsonCollectionQueryPolicy,
  type CollectionControl,
  type CollectionControlValue,
  type CollectionPanelMessages,
  type CollectionWorkflow
} from '@yueli/ui/collection'
import { CollectionPanel } from '@yueli/ui/collection/pattern'
import { useVueCollectionWorkflow } from '@yueli/ui/collection/vue'
import { createVueRouterCollectionQuerySync } from '@yueli/ui/collection/vue-router'
import { PASSWORD_HINT, passwordMeetsLengthPolicy } from '~/utils/password'
import { PageHeader } from '@yueli/ui/admin'
import { useMinimumLoading } from '@yueli/ui/feedback'
import { abs } from '~/utils/date'
import { createAccountNotifier } from '~/utils/feedback'
import type { MediaRef } from '~/utils/media'
import {
  adminStepUpFailureMessage,
  isAdminStepUpInterruptedError
} from '../../utils/admin-step-up-errors'

// 站群超级管理员·用户管理。identity 全局用户的全生命周期管理:列表/搜索/筛选/
// 分页 + 封禁/解封/删除 + 重置密码 + 建用户 + 授/撤 admin 角色。
definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '用户管理 · 控制台' })

const { me } = useSession()
const { call } = useApi()
const adminStepUp = useAdminStepUp()
const toast = createAccountNotifier(useToast())
const router = useRouter()

interface AdminUser {
  userKey: string
  email: string
  emailVerified: boolean
  status: 'active' | 'disabled' | 'deleted'
  createdAt: string
  displayName: string
  handle: string
  avatar?: MediaRef
  avatarUrl?: string
  roles: string[]
}
interface UserListData {
  list: AdminUser[]
  total: number
}
interface UserStats {
  total: number
  active: number
  disabled: number
  deleted: number
}

// ── Filters / paging ─────────────────────────────────────────────────────────
const ALL = '__all__' as const
type UserStatus = typeof ALL | AdminUser['status']
type UserRole = typeof ALL | 'user' | 'admin'
type UserSort = 'createdAt' | 'displayName'
type UserDirection = 'asc' | 'desc'
interface UserCollectionQuery {
  q: string
  status: UserStatus
  role: UserRole
  sort: UserSort
  direction: UserDirection
  page: number
  size: number
}

const statuses = [ALL, 'active', 'disabled', 'deleted'] as const
const roles = [ALL, 'user', 'admin'] as const
const sorts = ['createdAt', 'displayName'] as const
const pageSizes = [20, 50, 100] as const
const defaultQuery: UserCollectionQuery = { q: '', status: ALL, role: ALL, sort: 'createdAt', direction: 'desc', page: 1, size: 20 }
const queryPolicy = createJsonCollectionQueryPolicy<UserCollectionQuery>()
const searchInput = ref('')
const sync = createVueRouterCollectionQuerySync({
  router,
  codec: createCollectionRouteQueryCodec({
    q: { kind: 'string', default: defaultQuery.q, maxLength: 200 },
    status: { kind: 'enum', values: statuses, default: defaultQuery.status },
    role: { kind: 'enum', values: roles, default: defaultQuery.role },
    sort: { kind: 'enum', values: sorts, default: defaultQuery.sort },
    direction: { kind: 'enum', values: ['asc', 'desc'] as const, default: defaultQuery.direction },
    page: { kind: 'positive-integer', default: defaultQuery.page },
    size: { kind: 'positive-integer', values: pageSizes, default: defaultQuery.size }
  })
})

function isSelf(user: AdminUser) {
  return user.userKey === me.value?.userKey
}
async function load(nextQuery: Readonly<UserCollectionQuery>, activeWorkflow: CollectionWorkflow<AdminUser, string, UserCollectionQuery>) {
  const token = activeWorkflow.beginLoad()
  try {
    const data = await call<UserListData>('/api/v1/admin/users', {
      params: {
        page: nextQuery.page,
        size: nextQuery.size,
        keyword: nextQuery.q || undefined,
        status: nextQuery.status !== ALL ? nextQuery.status : undefined,
        role: nextQuery.role !== ALL ? nextQuery.role : undefined,
        orderBy: nextQuery.sort === 'displayName' ? 'display_name' : 'created_at',
        order: nextQuery.direction
      }
    })
    const lastPage = Math.max(1, Math.ceil((data.total ?? 0) / nextQuery.size))
    if (nextQuery.page > lastPage) {
      activeWorkflow.setQuery({ ...nextQuery, page: lastPage })
      return
    }
    activeWorkflow.resolveLoad(token, {
      items: (data.list ?? []).map(user => ({
        ...user,
        avatarUrl: userMediaUrl(user.avatar, 'thumbnail')
      })),
      total: data.total ?? 0
    })
  } catch {
    activeWorkflow.rejectLoad(token, { key: 'account.users.collection.load_failed' })
  }
}

const {
  snapshot: collection,
  workflow,
  reload
} = useVueCollectionWorkflow({
  initialQuery: defaultQuery,
  queryPolicy,
  keyOf: (user: AdminUser) => user.userKey,
  isSelectable: (user: AdminUser) => !isSelf(user),
  querySync: sync,
  dataQueryKey: (query) => JSON.stringify(query),
  load
})
const query = computed(() => collection.value.query)
function updateQuery(patch: Partial<UserCollectionQuery>, resetPage = true) {
  workflow.setQuery({ ...query.value, ...patch, ...(resetPage ? { page: 1 } : {}) })
}
const q = computed(() => query.value.q)
const status = computed({ get: () => query.value.status, set: (value: UserStatus) => updateQuery({ status: value }) })
const role = computed({ get: () => query.value.role, set: (value: UserRole) => updateQuery({ role: value }) })
const sort = computed({ get: () => query.value.sort, set: (value: UserSort) => updateQuery({ sort: value }) })
const direction = computed({ get: () => query.value.direction, set: (value: UserDirection) => updateQuery({ direction: value }) })
const page = computed({ get: () => query.value.page, set: (value: number) => updateQuery({ page: value }, false) })
const size = computed({ get: () => query.value.size, set: (value: number) => updateQuery({ size: value }) })

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(searchInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => updateQuery({ q: value.trim() }), 300)
})
searchInput.value = collection.value.query.q
watch(q, (value) => {
  if (searchInput.value !== value) searchInput.value = value
})

const users = computed(() => collection.value.items)
const total = computed(() => collection.value.total)
const stats = ref<UserStats>({ total: 0, active: 0, disabled: 0, deleted: 0 })

const mounted = ref(false)
onMounted(() => {
  mounted.value = true
  fetchStats()
})
watch(
  () => me.value?.userKey,
  (userKey, previousUserKey) => {
    if (mounted.value && userKey && userKey !== previousUserKey) void reload()
  }
)
onScopeDispose(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
const pending = computed(() => collection.value.loadState === 'loading' || collection.value.loadState === 'refreshing')
const showSkeleton = useMinimumLoading(computed(() => !mounted.value || pending.value))
async function fetchStats() {
  try {
    stats.value = await call<UserStats>('/api/v1/admin/users/stats')
  } catch {
    /* silent */
  }
}
async function reloadAll() {
  await Promise.all([reload(), fetchStats()])
}

function notifyStepUpFailure(error: unknown, title: string, fallback: string) {
  const description = adminStepUpFailureMessage(error, fallback)
  if (!description) return
  toast.add({ title, description, color: 'error' })
}

// ── Selection ────────────────────────────────────────────────────────────────
const selectedIds = computed(() => (collection.value.selection.mode === 'keys' ? collection.value.selection.keys : []))
const selectionCount = computed(() => collection.value.selection.count)
const isPageSelected = computed(() => collection.value.isPageSelected)
const isPageIndeterminate = computed(() => collection.value.isPageIndeterminate)
const isSelected = (id: string) => workflow.isSelected(id)
const toggleOne = (id: string, selected?: boolean) => {
  if (selected === undefined || selected !== workflow.isSelected(id)) workflow.toggleKey(id)
}
const togglePage = (selected?: boolean | 'indeterminate') => workflow.togglePage(selected === true)
const clearSelection = () => workflow.clearSelection()
function replaceSelection(ids: readonly string[]) {
  workflow.clearSelection()
  for (const id of ids) workflow.toggleKey(id)
}

// ── Row actions ──────────────────────────────────────────────────────────────
const busy = ref('') // id currently mutating

async function setStatus(u: AdminUser, next: AdminUser['status']) {
  busy.value = u.userKey
  try {
    await adminStepUp.run(
      'identity.admin.status.update',
      adminStepUpResource.status(u.userKey, next),
      (proof) => call(`/api/v1/admin/users/${u.userKey}/status`, {
        method: 'PUT', body: { status: next },
        headers: { 'X-Step-Up-Proof': proof },
      }),
    )
    if (workflow.isSelected(u.userKey)) workflow.toggleKey(u.userKey)
    await reloadAll()
  } catch (e) {
    notifyStepUpFailure(e, '操作失败', '暂时无法更新账户状态。')
  } finally {
    busy.value = ''
  }
}
const ban = (u: AdminUser) => setStatus(u, 'disabled')
const unban = (u: AdminUser) => setStatus(u, 'active')

function rowActions(u: AdminUser) {
  const items: { label: string; icon: string; color?: 'error'; disabled?: boolean; onSelect: () => void }[][] = [
    [
      { label: '查看详情', icon: 'i-tabler-eye', onSelect: () => openDetail(u) },
      {
        label: '重置密码',
        icon: 'i-tabler-lock',
        onSelect: () => {
          resetTarget.value = u
          newPw.value = ''
        }
      }
    ]
  ]
  const lifecycle: (typeof items)[number] = []
  if (u.status === 'active') lifecycle.push({ label: '封禁', icon: 'i-tabler-ban', color: 'error', disabled: isSelf(u), onSelect: () => ban(u) })
  if (u.status === 'disabled') lifecycle.push({ label: '解封', icon: 'i-tabler-circle-check', onSelect: () => unban(u) })
  if (u.status !== 'deleted')
    lifecycle.push({
      label: '删除',
      icon: 'i-tabler-trash',
      color: 'error',
      disabled: isSelf(u),
      onSelect: () => {
        deleteTarget.value = u
      }
    })
  items.push(lifecycle)
  return items
}

// ── Batch ────────────────────────────────────────────────────────────────────
const batchAction = ref<'' | 'active' | 'disabled'>('')
const batchRunning = ref(false)
const batchResult = ref<{ success: number; failed: number } | null>(null)
watch([q, status, role, sort, direction, page, size], () => {
  batchAction.value = ''
  batchResult.value = null
})
async function applyBatch() {
  if (!batchAction.value || !selectedIds.value.length || batchRunning.value) return
  const ids = selectedIds.value.filter((id) => id !== me.value?.userKey)
  batchRunning.value = true
  try {
    const results: PromiseSettledResult<unknown>[] = []
    let interruption: unknown
    for (const id of ids) {
      const next = batchAction.value
      const result = await Promise.resolve(adminStepUp.run(
        'identity.admin.status.update',
        adminStepUpResource.status(id, next),
        (proof) => call(`/api/v1/admin/users/${id}/status`, {
          method: 'PUT', body: { status: next },
          headers: { 'X-Step-Up-Proof': proof },
        }),
      )).then(
        (value): PromiseFulfilledResult<unknown> => ({ status: 'fulfilled', value }),
        (reason): PromiseRejectedResult => ({ status: 'rejected', reason }),
      )
      results.push(result)
      if (result.status === 'rejected' && isAdminStepUpInterruptedError(result.reason)) {
        interruption = result.reason
        break
      }
    }
    if (interruption) {
      notifyStepUpFailure(interruption, '验证已过期', '暂时无法批量更新账户状态。')
      return
    }
    const failedIds = ids.filter((_, index) => results[index]?.status === 'rejected')
    batchResult.value = { success: ids.length - failedIds.length, failed: failedIds.length }
    if (failedIds.length) replaceSelection(failedIds)
    else clearSelection()
    batchAction.value = ''
    await reloadAll()
  } finally {
    batchRunning.value = false
  }
}

// ── Detail modal (+ role toggle) ─────────────────────────────────────────────
const detailUser = ref<AdminUser | null>(null)
const detailOpen = computed({
  get: () => !!detailUser.value,
  set: (v) => {
    if (!v) detailUser.value = null
  }
})
function openDetail(u: AdminUser) {
  detailUser.value = { ...u }
}
const togglingRole = ref(false)
async function toggleAdminRole() {
  const u = detailUser.value
  if (!u) return
  const has = u.roles.includes('admin')
  togglingRole.value = true
  try {
    const action = has ? 'identity.admin.role.revoke' : 'identity.admin.role.grant'
    await adminStepUp.run(action, adminStepUpResource.role(u.userKey, 'admin'), (proof) =>
	  has
		? call(`/api/v1/admin/users/${u.userKey}/roles/admin`, {
			method: 'DELETE', headers: { 'X-Step-Up-Proof': proof },
		  })
		: call(`/api/v1/admin/users/${u.userKey}/roles`, {
            method: 'POST', body: { role: 'admin' },
            headers: { 'X-Step-Up-Proof': proof },
          }),
    )
    u.roles = has ? u.roles.filter((r) => r !== 'admin') : [...u.roles, 'admin']
    await reload()
  } catch (e) {
    notifyStepUpFailure(e, '操作失败', '暂时无法更新账户角色。')
  } finally {
    togglingRole.value = false
  }
}

// ── Reset password modal ─────────────────────────────────────────────────────
const resetTarget = ref<AdminUser | null>(null)
const resetOpen = computed({
  get: () => !!resetTarget.value,
  set: (v) => {
    if (!v) resetTarget.value = null
  }
})
const newPw = ref('')
const resetting = ref(false)
async function confirmReset() {
  if (!resetTarget.value || !passwordMeetsLengthPolicy(newPw.value)) return
  resetting.value = true
  try {
    const target = resetTarget.value
    await adminStepUp.run(
      'identity.admin.password.reset',
      adminStepUpResource.identity(target.userKey),
      (proof) => call(`/api/v1/admin/users/${target.userKey}/password`, {
        method: 'POST', body: { newPassword: newPw.value },
        headers: { 'X-Step-Up-Proof': proof },
      }),
    )
    resetTarget.value = null
  } catch (e) {
    notifyStepUpFailure(e, '重置失败', '暂时无法重置密码。')
  } finally {
    resetting.value = false
  }
}

// ── Delete confirm modal ─────────────────────────────────────────────────────
const deleteTarget = ref<AdminUser | null>(null)
const deleteOpen = computed({
  get: () => !!deleteTarget.value,
  set: (v) => {
    if (!v) deleteTarget.value = null
  }
})
const deleting = ref(false)
async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const target = deleteTarget.value
    await adminStepUp.run(
      'identity.admin.user.delete',
      adminStepUpResource.identity(target.userKey),
      (proof) => call(`/api/v1/admin/users/${target.userKey}`, {
        method: 'DELETE', headers: { 'X-Step-Up-Proof': proof },
      }),
    )
    deleteTarget.value = null
    await reloadAll()
  } catch (e) {
    notifyStepUpFailure(e, '删除失败', '暂时无法删除该账户。')
  } finally {
    deleting.value = false
  }
}

// ── Create user modal ────────────────────────────────────────────────────────
const createOpen = ref(false)
const createForm = reactive({ email: '', password: '', displayName: '', admin: false })
const creating = ref(false)
const createError = ref('')
function openCreate() {
  createForm.email = ''
  createForm.password = ''
  createForm.displayName = ''
  createForm.admin = false
  createError.value = ''
  createOpen.value = true
}
async function confirmCreate() {
  if (!createForm.email || !passwordMeetsLengthPolicy(createForm.password)) {
    createError.value = `请填写邮箱和 ${PASSWORD_MIN_LENGTH}–${PASSWORD_MAX_LENGTH} 个字符的密码。`
    return
  }
  createError.value = ''
  creating.value = true
  try {
    const roles = createForm.admin ? ['admin'] : []
    await adminStepUp.run(
      'identity.admin.user.create',
      adminStepUpResource.create(createForm.email, roles),
      (proof) => call('/api/v1/admin/users', {
        method: 'POST',
        headers: { 'X-Step-Up-Proof': proof },
        body: {
          email: createForm.email,
          password: createForm.password,
          displayName: createForm.displayName,
          roles,
        },
      }),
    )
    createOpen.value = false
    page.value = 1
    await reloadAll()
  } catch (e) {
    createError.value = adminStepUpFailureMessage(e, '暂时无法创建账户。') ?? ''
  } finally {
    creating.value = false
  }
}

// ── Helpers ──────────────────────────────────────────────────────────────────
const roleFilterItems = [
  { label: '全部角色', value: ALL },
  { label: '普通用户', value: 'user' },
  { label: '管理员', value: 'admin' }
]
const sortItems = [
  { label: '注册时间', value: 'createdAt' },
  { label: '昵称', value: 'displayName' }
]
const statusItems = computed(() => [
  { label: `全部状态 (${stats.value.active + stats.value.disabled})`, value: ALL },
  { label: `正常 (${stats.value.active})`, value: 'active' },
  { label: `已封禁 (${stats.value.disabled})`, value: 'disabled' },
  { label: `已删除 (${stats.value.deleted})`, value: 'deleted' }
])
const activeFilterCount = computed(() => Number(status.value !== defaultQuery.status) + Number(role.value !== ALL))
const collectionControls = computed<CollectionControl[]>(() => [
  { kind: 'select', id: 'status', label: '用户状态', section: '筛选条件', value: status.value, options: statusItems.value, icon: 'i-tabler-user-check', class: 'w-36' },
  { kind: 'select', id: 'role', label: '用户角色', section: '筛选条件', value: role.value, options: roleFilterItems, icon: 'i-tabler-shield', class: 'w-32' },
  { kind: 'select', id: 'sort', label: '排序方式', section: '列表排序', value: sort.value, options: sortItems, icon: 'i-tabler-arrows-sort', class: 'w-32' },
  { kind: 'direction', id: 'direction', label: '排序方向', section: '列表排序', value: direction.value, ascendingLabel: '升序', descendingLabel: '降序' }
])
const collectionMessages: CollectionPanelMessages = {
  searchPlaceholder: '搜索昵称、用户名或邮箱…',
  searchAction: '搜索',
  filtersAction: '筛选',
  activeFilters: (count) => `筛选（${count}）`,
  clearFilters: '重置',
  selectPage: '选择当前页可操作用户',
  selectItem: (label) => `选择用户：${label}`,
  bulkRegion: '用户批量操作',
  selected: (count) => `已选择 ${count} 个用户`,
  selectAllResults: '选择全部结果',
  clearSelection: '取消选择',
  emptyTitle: '没有匹配的用户',
  emptyDescription: '请调整搜索、状态或角色筛选后重试。',
  errorTitle: '用户加载失败',
  retry: '重新加载',
  showing: (first, last, count) => `显示 ${first}–${last}，共 ${count} 个`,
  pageSize: '每页',
  pageSizeControl: '每页用户数量',
  pageSizeOption: (value) => `${value} 个`
}
function changeCollectionControl(id: string, value: CollectionControlValue) {
  if (id === 'status' && statuses.includes(value as UserStatus)) status.value = value as UserStatus
  if (id === 'role' && roles.includes(value as UserRole)) role.value = value as UserRole
  if (id === 'sort' && sorts.includes(value as UserSort)) sort.value = value as UserSort
  if (id === 'direction' && (value === 'asc' || value === 'desc')) direction.value = value
}
function submitCollectionSearch(value: string) {
  if (searchTimer) clearTimeout(searchTimer)
  searchInput.value = value
  updateQuery({ q: value.trim() })
}
function clearActiveFilters() {
  updateQuery({ status: defaultQuery.status, role: defaultQuery.role })
}
const statusBadge: Record<AdminUser['status'], { label: string; color: 'success' | 'error' | 'neutral' }> = {
  active: { label: '正常', color: 'success' },
  disabled: { label: '已封禁', color: 'error' },
  deleted: { label: '已删除', color: 'neutral' }
}
function initialOf(u: AdminUser) {
  return (u.displayName || u.email || '?').charAt(0).toUpperCase()
}
const userKey = (user: AdminUser) => user.userKey
const userLabel = (user: AdminUser) => user.displayName || user.email
</script>

<template>
  <div class="space-y-5">
    <PageHeader title="用户管理" icon="i-tabler-users">
      <template #actions>
        <UButton icon="i-tabler-user-plus" label="新建用户" @click="openCreate" />
      </template>
    </PageHeader>

    <CollectionPanel
      v-model:search="searchInput"
      filter-panel-title="筛选与排序"
      filter-panel-description="选择后立即应用。"
      :items="users"
      :item-key="userKey"
      :item-label="userLabel"
      :controls="collectionControls"
      :messages="collectionMessages"
      :state="collection.issue ? 'error' : showSkeleton ? 'loading' : 'ready'"
      :error-message="collection.issue?.key"
      :total="total"
      :page="page"
      :page-size="size"
      :page-sizes="pageSizes"
      :active-filter-count="activeFilterCount"
      selectable
      :is-item-selectable="(user) => !isSelf(user)"
      :selection-count="selectionCount"
      :page-selected="isPageSelected"
      :page-indeterminate="isPageIndeterminate"
      :is-selected="isSelected"
      label="用户列表"
      @search="submitCollectionSearch"
      @control-change="changeCollectionControl"
      @clear-filters="clearActiveFilters"
      @retry="reload"
      @toggle-page="togglePage"
      @toggle-item="toggleOne"
      @clear-selection="clearSelection"
      @page-change="
        (value) => {
          page = value
        }
      "
      @page-size-change="
        (value) => {
          size = value
        }
      "
    >
      <template #columns>
        <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
          <span>用户、身份与注册时间</span>
          <span class="hidden w-40 text-right sm:block">状态与操作</span>
        </div>
      </template>

      <template #bulk-actions>
        <USelect
          v-model="batchAction"
          :items="[
            { label: '设为正常', value: 'active' },
            { label: '封禁', value: 'disabled' }
          ]"
          placeholder="批量操作"
          value-key="value"
          class="w-28"
          size="xs"
        />
        <UButton color="primary" variant="soft" size="xs" :loading="batchRunning" :disabled="!batchAction" @click="applyBatch">应用</UButton>
      </template>

      <template #item="{ item: u }">
        <div class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 sm:gap-3">
          <div class="flex min-w-0 items-center gap-3">
            <UAvatar :src="u.avatarUrl || undefined" :text="initialOf(u)" size="md" class="hidden shrink-0 sm:flex" />
            <div class="min-w-0 flex-1">
              <div class="flex min-w-0 items-center gap-2">
                <span class="line-clamp-1 text-sm font-medium text-highlighted">{{ u.displayName || '未命名' }}</span>
                <span v-if="isSelf(u)" class="shrink-0 rounded bg-primary/10 px-1.5 text-xs text-primary">你</span>
              </div>
              <div class="line-clamp-1 text-xs text-muted">
                {{ u.email }}<span v-if="u.handle"> · @{{ u.handle }}</span>
              </div>
              <div class="mt-0.5 text-xs text-dimmed">
                注册于 <ClientOnly fallback="—">{{ abs(u.createdAt) }}</ClientOnly>
              </div>
            </div>
          </div>
          <div class="flex shrink-0 items-center justify-end gap-1 sm:w-40 sm:gap-2">
            <div class="hidden flex-wrap items-center gap-1 min-[390px]:flex">
              <UBadge :label="statusBadge[u.status].label" :color="statusBadge[u.status].color" variant="subtle" size="sm" />
              <UBadge v-if="u.roles.includes('admin')" label="管理员" color="warning" variant="soft" size="sm" class="hidden lg:inline-flex" />
            </div>
            <UDropdownMenu :items="rowActions(u)">
              <UButton
                color="neutral"
                variant="ghost"
                icon="i-tabler-dots-vertical"
                square
                size="xs"
                class="min-h-11 min-w-11 touch-manipulation sm:min-h-0 sm:min-w-0"
                :loading="busy === u.userKey"
                :aria-label="`管理 ${u.displayName || u.email}`"
              />
            </UDropdownMenu>
          </div>
        </div>
      </template>
    </CollectionPanel>

    <UAlert
      v-if="batchResult"
      class="mt-3"
      :color="batchResult.failed ? 'warning' : 'success'"
      variant="subtle"
      :icon="batchResult.failed ? 'i-tabler-alert-triangle' : 'i-tabler-circle-check'"
      :title="`已处理 ${batchResult.success} 个用户`"
      :description="batchResult.failed ? `${batchResult.failed} 个用户处理失败，已保留选中。` : '批量操作已完成。'"
      close
      @update:open="batchResult = null"
    />

    <!-- ── Detail modal ── -->
    <UModal v-model:open="detailOpen" :title="detailUser?.displayName || '用户详情'">
      <template #body>
        <div v-if="detailUser" class="space-y-4">
          <div class="flex items-center gap-4">
            <UAvatar :src="detailUser.avatarUrl || undefined" :text="initialOf(detailUser)" size="lg" />
            <div class="min-w-0">
              <p class="truncate font-medium text-highlighted">{{ detailUser.displayName || '未命名' }}</p>
              <p class="truncate text-sm text-muted">{{ detailUser.email }}</p>
            </div>
          </div>
          <dl class="grid grid-cols-3 gap-y-2 text-sm">
            <dt class="text-muted">用户名</dt>
            <dd class="col-span-2 text-default">{{ detailUser.handle || '—' }}</dd>
            <dt class="text-muted">状态</dt>
            <dd class="col-span-2"><UBadge :label="statusBadge[detailUser.status].label" :color="statusBadge[detailUser.status].color" variant="soft" size="sm" /></dd>
            <dt class="text-muted">邮箱验证</dt>
            <dd class="col-span-2 text-default">{{ detailUser.emailVerified ? '已验证' : '未验证' }}</dd>
            <dt class="text-muted">注册时间</dt>
            <dd class="col-span-2 text-default">
              <ClientOnly fallback="—">{{ abs(detailUser.createdAt) }}</ClientOnly>
            </dd>
            <dt class="text-muted">用户 ID</dt>
            <dd class="col-span-2 truncate font-mono text-xs text-dimmed">{{ detailUser.userKey }}</dd>
          </dl>
          <div class="flex items-center justify-between gap-4 border-t border-default pt-4">
            <div>
              <p class="text-sm font-medium text-default">管理员权限</p>
              <p class="text-xs text-muted">仅授予 Identity 与基础服务控制面；各产品站权限仍由实例本地角色决定</p>
            </div>
            <UButton
              :color="detailUser.roles.includes('admin') ? 'error' : 'primary'"
              :variant="detailUser.roles.includes('admin') ? 'soft' : 'solid'"
              size="sm"
              :loading="togglingRole"
              :disabled="isSelf(detailUser)"
              :label="detailUser.roles.includes('admin') ? '撤销管理员' : '授予管理员'"
              @click="toggleAdminRole"
            />
          </div>
        </div>
      </template>
    </UModal>

    <!-- ── Reset password modal ── -->
    <UModal v-model:open="resetOpen" title="重置密码">
      <template #body>
        <div class="space-y-3">
          <p class="text-sm text-muted">
            为 <span class="font-medium text-default">{{ resetTarget?.email }}</span> 设置新密码,该用户其他会话将被退出。
          </p>
          <UInput v-model="newPw" type="password" :placeholder="PASSWORD_HINT" class="w-full" autocomplete="new-password" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="resetting"
            @click="
              () => {
                resetTarget = null
              }
            "
          />
          <UButton color="primary" label="重置密码" :loading="resetting" :disabled="!passwordMeetsLengthPolicy(newPw)" @click="confirmReset" />
        </div>
      </template>
    </UModal>

    <!-- ── Delete confirm ── -->
    <UModal v-model:open="deleteOpen" title="删除用户?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ deleteTarget?.email }}</span
          >。该账户将无法登录,其邮箱可被重新注册。此操作不可轻易撤销。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="deleting"
            @click="
              () => {
                deleteTarget = null
              }
            "
          />
          <UButton color="error" label="确认删除" :loading="deleting" @click="confirmDelete" />
        </div>
      </template>
    </UModal>

    <!-- ── Create user ── -->
    <UModal v-model:open="createOpen" title="新建用户">
      <template #body>
        <div class="space-y-4">
          <UAlert v-if="createError" color="error" variant="soft" icon="i-tabler-alert-circle" :description="createError" />
          <UFormField label="邮箱" required>
            <UInput v-model="createForm.email" type="email" placeholder="user@example.com" class="w-full" />
          </UFormField>
          <UFormField label="初始密码" :hint="PASSWORD_HINT" required>
            <UInput v-model="createForm.password" type="password" class="w-full" autocomplete="new-password" />
          </UFormField>
          <UFormField label="昵称">
            <UInput v-model="createForm.displayName" placeholder="可选" class="w-full" />
          </UFormField>
          <UCheckbox v-model="createForm.admin" label="授予管理员权限" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="creating"
            @click="
              () => {
                createOpen = false
              }
            "
          />
          <UButton color="primary" label="创建" :loading="creating" @click="confirmCreate" />
        </div>
      </template>
    </UModal>
  </div>
</template>
