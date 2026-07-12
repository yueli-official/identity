<script setup lang="ts">
import {
  ManageHeader, ManageTabs, ManageEmpty, ManagePagination, ManageCollectionToolbar, ManageCollectionDock, ManagePageSelection, SkeletonList
} from '@platform/manage/components'
import { manageCollectionQueryFingerprint, serializeManageCollectionQuery, type ManageCollectionDefinition } from '@platform/manage/collection'
import { useManageCollectionState } from '@platform/manage/use-manage-collection-state'
import { useManageSelection } from '@platform/manage/use-manage-selection'
import { useMinLoading } from '@platform/ui/use-min-loading'
import { abs } from '@platform/ui/date'
import { createPlatformNotifier } from '@platform/ui/feedback'

// 站群超级管理员·用户管理。identity 全局用户的全生命周期管理:列表/搜索/筛选/
// 分页 + 封禁/解封/删除 + 重置密码 + 建用户 + 授/撤 admin 角色。
definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '用户管理 · 控制台' })

const { me } = useSession()
const { call } = useApi()
const toast = createPlatformNotifier(useToast())
const route = useRoute()
const router = useRouter()

interface AdminUser {
  id: string
  email: string
  emailVerified: boolean
  status: 'active' | 'disabled' | 'deleted'
  createdAt: string
  displayName: string
  username: string
  avatarUrl: string
  roles: string[]
}
interface UserListData { list: AdminUser[], total: number }
interface UserStats { total: number, active: number, disabled: number, deleted: number }

// ── Filters / paging ─────────────────────────────────────────────────────────
const ALL = '__all__'
const collectionDefinition = {
  resourceKind: 'account-user',
  statuses: ['', 'active', 'disabled', 'deleted'],
  views: ['list'],
  sortKeys: ['createdAt', 'displayName'],
  pageSizes: [20, 50, 100],
  defaultStatus: '', defaultView: 'list', defaultSort: 'createdAt', defaultDirection: 'desc', defaultPageSize: 20,
  pagination: 'server', selection: 'page', filters: ['role']
} as const satisfies ManageCollectionDefinition
const { searchInput, q, status, sort, direction, page, size, state: collectionState, filterModel } = useManageCollectionState({
  definition: collectionDefinition,
  routeQuery: computed(() => route.query),
  replaceQuery: query => router.replace({ query })
})
const role = filterModel('role', ALL)

const users = ref<AdminUser[]>([])
const total = ref(0)
const stats = ref<UserStats>({ total: 0, active: 0, disabled: 0, deleted: 0 })

const mounted = ref(false)
const loading = ref(true)
onMounted(() => { mounted.value = true; fetchUsers(); fetchStats() })
const showSkeleton = useMinLoading(computed(() => !mounted.value || loading.value))

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

const statusTabs = computed(() => [
  { key: '', label: '全部', count: stats.value.active + stats.value.disabled },
  { key: 'active', label: '正常', count: stats.value.active },
  { key: 'disabled', label: '已封禁', count: stats.value.disabled },
  { key: 'deleted', label: '已删除', count: stats.value.deleted }
])

async function fetchUsers() {
  loading.value = true
  try {
    const data = await call<UserListData>('/api/v1/admin/users', {
      params: {
        page: page.value,
        size: size.value,
        keyword: q.value || undefined,
        status: status.value || undefined,
        role: role.value !== ALL ? role.value : undefined,
        orderBy: sort.value === 'displayName' ? 'display_name' : 'created_at',
        order: direction.value
      }
    })
    users.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    toast.add({ title: '加载用户失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    loading.value = false
  }
}
async function fetchStats() {
  try { stats.value = await call<UserStats>('/api/v1/admin/users/stats') } catch { /* silent */ }
}
async function reloadAll() { await Promise.all([fetchUsers(), fetchStats()]) }

watch([q, status, role, sort, direction, page, size], fetchUsers)

// ── Selection ────────────────────────────────────────────────────────────────
const selectionResetKey = computed(() => manageCollectionQueryFingerprint(serializeManageCollectionQuery(collectionState.value, collectionDefinition)))
const { selectedIds, selectionCount, isPageSelected, isPageIndeterminate, isSelected, toggleOne, togglePage, keepOnly, clear: clearSelection } = useManageSelection({
  visibleIds: computed(() => users.value.filter(user => !isSelf(user)).map(user => user.id)), filteredTotal: total, resetKey: selectionResetKey
})

// ── Row actions ──────────────────────────────────────────────────────────────
const busy = ref('') // id currently mutating
function isSelf(user: AdminUser) { return user.id === me.value?.id }

async function setStatus(u: AdminUser, next: AdminUser['status']) {
  busy.value = u.id
  try {
    await call(`/api/v1/admin/users/${u.id}/status`, { method: 'PUT', body: { status: next } })
    u.status = next
    await fetchStats()
  } catch (e) {
    toast.add({ title: '操作失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    busy.value = ''
  }
}
const ban = (u: AdminUser) => setStatus(u, 'disabled')
const unban = (u: AdminUser) => setStatus(u, 'active')

function rowActions(u: AdminUser) {
  const items: { label: string, icon: string, color?: 'error', disabled?: boolean, onSelect: () => void }[][] = [[
    { label: '查看详情', icon: 'i-tabler-eye', onSelect: () => openDetail(u) },
    { label: '重置密码', icon: 'i-tabler-lock', onSelect: () => { resetTarget.value = u; newPw.value = '' } }
  ]]
  const lifecycle: typeof items[number] = []
  if (u.status === 'active') lifecycle.push({ label: '封禁', icon: 'i-tabler-ban', color: 'error', disabled: isSelf(u), onSelect: () => ban(u) })
  if (u.status === 'disabled') lifecycle.push({ label: '解封', icon: 'i-tabler-circle-check', onSelect: () => unban(u) })
  if (u.status !== 'deleted') lifecycle.push({ label: '删除', icon: 'i-tabler-trash', color: 'error', disabled: isSelf(u), onSelect: () => { deleteTarget.value = u } })
  items.push(lifecycle)
  return items
}

// ── Batch ────────────────────────────────────────────────────────────────────
const batchAction = ref<'' | 'active' | 'disabled'>('')
const batchRunning = ref(false)
const batchResult = ref<{ success: number, failed: number } | null>(null)
async function applyBatch() {
  if (!batchAction.value || !selectedIds.value.length || batchRunning.value) return
  const ids = selectedIds.value.filter(id => id !== me.value?.id)
  batchRunning.value = true
  try {
    const results = await Promise.allSettled(ids.map(id => call(`/api/v1/admin/users/${id}/status`, { method: 'PUT', body: { status: batchAction.value } })))
    const failedIds = ids.filter((_, index) => results[index]?.status === 'rejected')
    batchResult.value = { success: ids.length - failedIds.length, failed: failedIds.length }
    if (failedIds.length) keepOnly(failedIds)
    else clearSelection()
    batchAction.value = ''
    await reloadAll()
  } finally {
    batchRunning.value = false
  }
}

// ── Detail modal (+ role toggle) ─────────────────────────────────────────────
const detailUser = ref<AdminUser | null>(null)
const detailOpen = computed({ get: () => !!detailUser.value, set: v => { if (!v) detailUser.value = null } })
function openDetail(u: AdminUser) { detailUser.value = { ...u } }
const togglingRole = ref(false)
async function toggleAdminRole() {
  const u = detailUser.value
  if (!u) return
  const has = u.roles.includes('admin')
  togglingRole.value = true
  try {
    if (has) await call(`/api/v1/admin/identities/${u.id}/roles/admin`, { method: 'DELETE' })
    else await call(`/api/v1/admin/identities/${u.id}/roles`, { method: 'POST', body: { role: 'admin' } })
    u.roles = has ? u.roles.filter(r => r !== 'admin') : [...u.roles, 'admin']
    await fetchUsers()
  } catch (e) {
    toast.add({ title: '操作失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    togglingRole.value = false
  }
}

// ── Reset password modal ─────────────────────────────────────────────────────
const resetTarget = ref<AdminUser | null>(null)
const resetOpen = computed({ get: () => !!resetTarget.value, set: v => { if (!v) resetTarget.value = null } })
const newPw = ref('')
const resetting = ref(false)
async function confirmReset() {
  if (!resetTarget.value || newPw.value.length < 8) return
  resetting.value = true
  try {
    await call(`/api/v1/admin/users/${resetTarget.value.id}/password`, { method: 'POST', body: { newPassword: newPw.value } })
    resetTarget.value = null
  } catch (e) {
    toast.add({ title: '重置失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    resetting.value = false
  }
}

// ── Delete confirm modal ─────────────────────────────────────────────────────
const deleteTarget = ref<AdminUser | null>(null)
const deleteOpen = computed({ get: () => !!deleteTarget.value, set: v => { if (!v) deleteTarget.value = null } })
const deleting = ref(false)
async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await call(`/api/v1/admin/users/${deleteTarget.value.id}`, { method: 'DELETE' })
    deleteTarget.value = null
    await reloadAll()
  } catch (e) {
    toast.add({ title: '删除失败', description: (e as Error)?.message, color: 'error' })
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
  createForm.email = ''; createForm.password = ''; createForm.displayName = ''; createForm.admin = false
  createError.value = ''
  createOpen.value = true
}
async function confirmCreate() {
  if (!createForm.email || createForm.password.length < 8) {
    createError.value = '请填写邮箱和至少 8 位密码。'
    return
  }
  createError.value = ''
  creating.value = true
  try {
    await call('/api/v1/admin/users', {
      method: 'POST',
      body: {
        email: createForm.email,
        password: createForm.password,
        displayName: createForm.displayName,
        roles: createForm.admin ? ['admin'] : []
      }
    })
    createOpen.value = false
    page.value = 1
    await reloadAll()
  } catch (e) {
    toast.add({ title: '创建失败', description: (e as Error)?.message, color: 'error' })
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
const pageSizeItems = [20, 50, 100].map(value => ({ label: `${value}/页`, value }))
const activeFilterCount = computed(() => role.value === ALL ? 0 : 1)
const statusBadge: Record<AdminUser['status'], { label: string, color: 'success' | 'error' | 'neutral' }> = {
  active: { label: '正常', color: 'success' },
  disabled: { label: '已封禁', color: 'error' },
  deleted: { label: '已删除', color: 'neutral' }
}
function initialOf(u: AdminUser) { return (u.displayName || u.email || '?').charAt(0).toUpperCase() }
</script>

<template>
  <div>
    <ManageHeader title="用户管理">
      <template #subtitle>站群全局账户 —— 封禁、删除、重置密码、授予管理员</template>
      <template #actions>
        <div class="flex items-center gap-3">
          <div class="hidden items-center gap-2 sm:flex">
            <div class="rounded-lg border border-default bg-default px-3 py-1.5 text-center">
              <div class="text-base font-semibold text-highlighted tabular-nums">{{ stats.total }}</div>
              <div class="text-xs text-muted">总用户</div>
            </div>
            <div class="rounded-lg border border-error/20 bg-error/5 px-3 py-1.5 text-center">
              <div class="text-base font-semibold text-error tabular-nums">{{ stats.disabled }}</div>
              <div class="text-xs text-error/80">已封禁</div>
            </div>
          </div>
          <UButton icon="i-tabler-user-plus" label="新建用户" @click="openCreate" />
        </div>
      </template>
    </ManageHeader>

    <ManageTabs v-model="status" :items="statusTabs" class="mb-4" />

    <ManageCollectionToolbar
      v-model:search="searchInput"
      search-placeholder="搜索昵称、用户名或邮箱…"
      :filter-count="activeFilterCount"
      class="mb-5"
    >
      <template #filters>
        <USelect v-model="role" :items="roleFilterItems" value-key="value" class="w-full" />
        <USelect v-model="sort" :items="sortItems" value-key="value" class="w-full" />
        <UButton
          color="neutral"
          variant="outline"
          :icon="direction === 'asc' ? 'i-tabler-sort-ascending' : 'i-tabler-sort-descending'"
          :label="direction === 'asc' ? '升序' : '降序'"
          @click="() => { direction = direction === 'asc' ? 'desc' : 'asc' }"
        />
      </template>
    </ManageCollectionToolbar>

    <!-- list -->
    <SkeletonList v-if="showSkeleton" :rows="8" />

    <ManageEmpty
      v-else-if="!users.length"
      icon="i-tabler-users"
      :text="q || activeFilterCount ? '没有匹配的用户' : '还没有用户'" />

    <ul v-else class="space-y-2.5">
      <li
        v-for="u in users" :key="u.id"
        class="group grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-xl border border-default bg-default px-3 py-3 transition sm:px-4 hover:shadow-sm">
        <UCheckbox :model-value="isSelected(u.id)" :disabled="isSelf(u)" @update:model-value="toggleOne(u.id)" />
        <div class="flex min-w-0 items-center gap-3">
          <UAvatar :src="u.avatarUrl || undefined" :text="initialOf(u)" size="md" class="hidden shrink-0 sm:flex" />
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 items-center gap-2">
              <span class="line-clamp-1 text-sm font-medium text-highlighted">{{ u.displayName || '未命名' }}</span>
              <span v-if="isSelf(u)" class="shrink-0 rounded bg-primary/10 px-1.5 text-xs text-primary">你</span>
            </div>
            <div class="line-clamp-1 text-xs text-muted">{{ u.email }}<span v-if="u.username"> · @{{ u.username }}</span></div>
            <div class="mt-0.5 text-xs text-dimmed">注册于 <ClientOnly fallback="—">{{ abs(u.createdAt) }}</ClientOnly></div>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <UBadge v-if="u.roles.includes('admin')" label="管理员" color="warning" variant="soft" size="sm" class="hidden sm:inline-flex" />
          <UDropdownMenu :items="rowActions(u)">
            <UButton
              color="neutral" variant="ghost" icon="i-tabler-dots-vertical" square size="xs"
              :loading="busy === u.id"
              :aria-label="`管理 ${u.displayName || u.email}`" />
          </UDropdownMenu>
        </div>
      </li>
    </ul>

    <ManageCollectionDock v-if="users.length" label="用户选择、批量操作与分页">
      <template #selection>
        <ManagePageSelection :model-value="isPageSelected" :indeterminate="isPageIndeterminate" label="选择当前页用户" @update:model-value="togglePage" />
        <template v-if="selectionCount">
          <span>已选 {{ selectionCount }}</span>
          <USeparator orientation="vertical" class="h-4" />
          <USelect
            v-model="batchAction"
            :items="[{ label: '设为正常', value: 'active' }, { label: '封禁', value: 'disabled' }]"
            placeholder="批量操作" value-key="value" class="w-32" size="sm" />
          <UButton color="primary" variant="soft" size="sm" :loading="batchRunning" :disabled="!batchAction" @click="applyBatch">应用</UButton>
          <UButton color="neutral" variant="ghost" size="sm" :disabled="batchRunning" @click="clearSelection(); batchAction = ''; batchResult = null">清除</UButton>
        </template>
        <span v-else class="text-xs">共 {{ total }} 个用户</span>
        <span v-if="batchResult" class="text-xs" :class="batchResult.failed ? 'text-warning' : 'text-success'">
          成功 {{ batchResult.success }}<template v-if="batchResult.failed">，失败 {{ batchResult.failed }}</template>
        </span>
      </template>
      <template #pagination>
        <USelect v-model="size" :items="pageSizeItems" value-key="value" size="sm" class="w-24" />
        <ManagePagination v-model="page" :total-pages="totalPages" />
      </template>
    </ManageCollectionDock>

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
            <dt class="text-muted">用户名</dt><dd class="col-span-2 text-default">{{ detailUser.username || '—' }}</dd>
            <dt class="text-muted">状态</dt>
            <dd class="col-span-2"><UBadge :label="statusBadge[detailUser.status].label" :color="statusBadge[detailUser.status].color" variant="soft" size="sm" /></dd>
            <dt class="text-muted">邮箱验证</dt>
            <dd class="col-span-2 text-default">{{ detailUser.emailVerified ? '已验证' : '未验证' }}</dd>
            <dt class="text-muted">注册时间</dt><dd class="col-span-2 text-default"><ClientOnly fallback="—">{{ abs(detailUser.createdAt) }}</ClientOnly></dd>
            <dt class="text-muted">用户 ID</dt><dd class="col-span-2 truncate font-mono text-xs text-dimmed">{{ detailUser.id }}</dd>
          </dl>
          <div class="flex items-center justify-between gap-4 border-t border-default pt-4">
            <div>
              <p class="text-sm font-medium text-default">管理员权限</p>
              <p class="text-xs text-muted">授予后该账户可访问站群所有管理后台</p>
            </div>
            <UButton
              :color="detailUser.roles.includes('admin') ? 'error' : 'primary'"
              :variant="detailUser.roles.includes('admin') ? 'soft' : 'solid'"
              size="sm" :loading="togglingRole" :disabled="isSelf(detailUser)"
              :label="detailUser.roles.includes('admin') ? '撤销管理员' : '授予管理员'"
              @click="toggleAdminRole" />
          </div>
        </div>
      </template>
    </UModal>

    <!-- ── Reset password modal ── -->
    <UModal v-model:open="resetOpen" title="重置密码">
      <template #body>
        <div class="space-y-3">
          <p class="text-sm text-muted">为 <span class="font-medium text-default">{{ resetTarget?.email }}</span> 设置新密码,该用户其他会话将被退出。</p>
          <UInput v-model="newPw" type="password" placeholder="新密码(至少 8 位)" class="w-full" autocomplete="new-password" />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="resetting" @click="() => { resetTarget = null }" />
          <UButton color="primary" label="重置密码" :loading="resetting" :disabled="newPw.length < 8" @click="confirmReset" />
        </div>
      </template>
    </UModal>

    <!-- ── Delete confirm ── -->
    <UModal v-model:open="deleteOpen" title="删除用户?">
      <template #body>
        <p class="text-sm text-muted">
          将删除 <span class="font-medium text-default">{{ deleteTarget?.email }}</span>。该账户将无法登录,其邮箱可被重新注册。此操作不可轻易撤销。
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="deleting" @click="() => { deleteTarget = null }" />
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
          <UFormField label="初始密码" hint="至少 8 位" required>
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="creating" @click="() => { createOpen = false }" />
          <UButton color="primary" label="创建" :loading="creating" @click="confirmCreate" />
        </div>
      </template>
    </UModal>
  </div>
</template>
