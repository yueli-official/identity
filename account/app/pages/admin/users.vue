<script setup lang="ts">
import {
  ManageHeader, ManageTabs, ManageEmpty, ManagePagination, ManagePageFooter, SkeletonList
} from '@platform/manage/components'
import { useMinLoading } from '@platform/ui/use-min-loading'

// 站群超级管理员·用户管理。identity 全局用户的全生命周期管理:列表/搜索/筛选/
// 分页 + 封禁/解封/删除 + 重置密码 + 建用户 + 授/撤 admin 角色。
definePageMeta({ layout: 'admin', middleware: 'admin' })
useSeoMeta({ title: '用户管理 · 控制台' })

const { me } = useSession()
const { call } = useApi()
const toast = useToast()

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
const status = ref<'' | 'active' | 'disabled' | 'deleted'>('')
const keyword = ref('')
const role = ref<'all' | 'user' | 'admin'>('all')
const sort = ref<'created_desc' | 'created_asc' | 'name_asc'>('created_desc')
const page = ref(1)
const SIZE = 20

const users = ref<AdminUser[]>([])
const total = ref(0)
const stats = ref<UserStats>({ total: 0, active: 0, disabled: 0, deleted: 0 })
const selected = ref<string[]>([])

const mounted = ref(false)
const loading = ref(true)
onMounted(() => { mounted.value = true; fetchUsers(); fetchStats() })
const showSkeleton = useMinLoading(computed(() => !mounted.value || loading.value))

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / SIZE)))

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
        size: SIZE,
        keyword: keyword.value || undefined,
        status: status.value || undefined,
        role: role.value !== 'all' ? role.value : undefined,
        orderBy: sort.value === 'name_asc' ? 'display_name' : 'created_at',
        order: sort.value === 'created_asc' ? 'asc' : 'desc'
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

// reset to page 1 + clear selection whenever a filter changes.
watch(status, () => { page.value = 1; selected.value = []; fetchUsers() })
watch([role, sort], () => { page.value = 1; selected.value = []; fetchUsers() })
watch(page, () => { selected.value = []; fetchUsers() })
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(keyword, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { page.value = 1; selected.value = []; fetchUsers() }, 350)
})

// ── Selection ────────────────────────────────────────────────────────────────
const allSelected = computed(() => users.value.length > 0 && users.value.every(u => selected.value.includes(u.id)))
const indeterminate = computed(() => selected.value.length > 0 && !allSelected.value)
function toggleAll(v: boolean | 'indeterminate') {
  selected.value = (v === false || allSelected.value) ? [] : users.value.map(u => u.id)
}
function toggleOne(id: string) {
  const i = selected.value.indexOf(id)
  if (i > -1) selected.value.splice(i, 1)
  else selected.value.push(id)
}

// ── Row actions ──────────────────────────────────────────────────────────────
const busy = ref('') // id currently mutating
const isSelf = (u: AdminUser) => u.id === me.value?.id

async function setStatus(u: AdminUser, next: AdminUser['status'], okMsg: string) {
  busy.value = u.id
  try {
    await call(`/api/v1/admin/users/${u.id}/status`, { method: 'PUT', body: { status: next } })
    toast.add({ title: okMsg, color: 'success', icon: 'i-tabler-check' })
    await reloadAll()
  } catch (e) {
    toast.add({ title: '操作失败', description: (e as Error)?.message, color: 'error' })
  } finally {
    busy.value = ''
  }
}
const ban = (u: AdminUser) => setStatus(u, 'disabled', '已封禁')
const unban = (u: AdminUser) => setStatus(u, 'active', '已解封')

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
async function applyBatch() {
  if (!batchAction.value || !selected.value.length) return
  const ids = selected.value.filter(id => id !== me.value?.id) // never self
  try {
    await Promise.all(ids.map(id => call(`/api/v1/admin/users/${id}/status`, { method: 'PUT', body: { status: batchAction.value } })))
    toast.add({ title: `已更新 ${ids.length} 个用户`, color: 'success', icon: 'i-tabler-check' })
    selected.value = []
    batchAction.value = ''
    await reloadAll()
  } catch (e) {
    toast.add({ title: '批量操作失败', description: (e as Error)?.message, color: 'error' })
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
    toast.add({ title: has ? '已撤销管理员' : '已授予管理员', color: 'success', icon: 'i-tabler-check' })
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
    toast.add({ title: '密码已重置', description: '该用户其他设备的会话已退出。', color: 'success', icon: 'i-tabler-lock' })
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
    toast.add({ title: '已删除用户', color: 'success', icon: 'i-tabler-check' })
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
function openCreate() {
  createForm.email = ''; createForm.password = ''; createForm.displayName = ''; createForm.admin = false
  createOpen.value = true
}
async function confirmCreate() {
  if (!createForm.email || createForm.password.length < 8) {
    toast.add({ title: '请填写邮箱和至少 8 位密码', color: 'warning' })
    return
  }
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
    toast.add({ title: '用户已创建', color: 'success', icon: 'i-tabler-user-plus' })
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
  { label: '全部角色', value: 'all' },
  { label: '普通用户', value: 'user' },
  { label: '管理员', value: 'admin' }
]
const sortItems = [
  { label: '最新注册', value: 'created_desc' },
  { label: '最早注册', value: 'created_asc' },
  { label: '昵称 A→Z', value: 'name_asc' }
]
const statusBadge: Record<AdminUser['status'], { label: string, color: 'success' | 'error' | 'neutral' }> = {
  active: { label: '正常', color: 'success' },
  disabled: { label: '已封禁', color: 'error' },
  deleted: { label: '已删除', color: 'neutral' }
}
function fmtDate(s: string) {
  if (!s) return '—'
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
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

    <!-- search + filters -->
    <div class="mb-5 flex flex-wrap items-center gap-3">
      <UInput
        v-model="keyword" placeholder="搜索昵称 / 用户名 / 邮箱"
        icon="i-tabler-search" class="w-full sm:max-w-xs" />
      <USelect v-model="role" :items="roleFilterItems" value-key="value" class="w-32" />
      <USelect v-model="sort" :items="sortItems" value-key="value" class="w-36" />
    </div>

    <!-- list -->
    <SkeletonList v-if="showSkeleton" :rows="8" />

    <ManageEmpty
      v-else-if="!users.length"
      icon="i-tabler-users"
      :text="keyword ? '没有匹配的用户' : '还没有用户'" />

    <ul v-else class="space-y-2.5">
      <li
        v-for="u in users" :key="u.id"
        class="group flex items-center gap-4 rounded-xl border border-default bg-default px-4 py-3 transition hover:shadow-sm">
        <UCheckbox :model-value="selected.includes(u.id)" @update:model-value="toggleOne(u.id)" />
        <UAvatar :src="u.avatarUrl || undefined" :text="initialOf(u)" size="md" class="shrink-0" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span class="truncate text-sm font-medium text-highlighted">{{ u.displayName || '未命名' }}</span>
            <span v-if="u.username" class="text-xs text-muted">@{{ u.username }}</span>
            <span v-if="isSelf(u)" class="rounded bg-primary/10 px-1.5 text-xs text-primary">你</span>
          </div>
          <div class="truncate text-xs text-muted">{{ u.email }}</div>
          <div class="mt-0.5 text-xs text-dimmed">注册于 {{ fmtDate(u.createdAt) }}</div>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <UBadge v-if="u.roles.includes('admin')" label="管理员" color="warning" variant="soft" size="sm" />
          <UBadge :label="statusBadge[u.status].label" :color="statusBadge[u.status].color" variant="soft" size="sm" />
          <UDropdownMenu :items="rowActions(u)">
            <UButton
              color="neutral" variant="ghost" icon="i-tabler-dots-vertical" square size="xs"
              :loading="busy === u.id"
              class="opacity-0 transition group-hover:opacity-100" />
          </UDropdownMenu>
        </div>
      </li>
    </ul>

    <!-- footer: batch + pagination -->
    <ManagePageFooter v-if="users.length">
      <template #left>
        <UCheckbox :model-value="allSelected" :indeterminate="indeterminate" @update:model-value="toggleAll" />
        <template v-if="selected.length">
          <span>已选 {{ selected.length }}</span>
          <USeparator orientation="vertical" class="h-4" />
          <USelect
            v-model="batchAction"
            :items="[{ label: '设为正常', value: 'active' }, { label: '封禁', value: 'disabled' }]"
            placeholder="批量操作" value-key="value" class="w-32" size="sm" />
          <UButton color="primary" variant="soft" size="sm" :disabled="!batchAction" @click="applyBatch">应用</UButton>
          <UButton color="neutral" variant="ghost" size="sm" @click="selected = []; batchAction = ''">取消</UButton>
        </template>
        <span v-else class="text-xs">共 {{ total }} 个用户</span>
      </template>
      <template #right>
        <ManagePagination v-model="page" :total-pages="totalPages" />
      </template>
    </ManagePageFooter>

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
            <dt class="text-muted">注册时间</dt><dd class="col-span-2 text-default">{{ fmtDate(detailUser.createdAt) }}</dd>
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
