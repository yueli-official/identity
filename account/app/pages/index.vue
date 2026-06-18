<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ middleware: 'auth' })

const { me, refresh, logout } = useSession()
const { call } = useApi()
const toast = useToast()

const initial = computed(() =>
  (me.value?.displayName || me.value?.email || '?').charAt(0).toUpperCase()
)

// ── Email verification ──────────────────────────────────────────────────────
const resending = ref(false)
async function onResendVerification() {
  resending.value = true
  try {
    await call('/api/v1/auth/email/verify-request', { method: 'POST' })
    toast.add({ title: '验证邮件已发送', description: '请检查邮箱完成验证。(demo 无真实邮件:链接打印在后端日志)', color: 'success', icon: 'i-tabler-mail-check' })
  } catch (err: any) {
    toast.add({ title: '发送失败', description: err?.data?.message || '请稍后重试。', color: 'error' })
  } finally {
    resending.value = false
  }
}

// ── Profile edit ────────────────────────────────────────────────────────────
const profileSchema = z.object({
  displayName: z.string().min(1, '请输入昵称'),
  username: z.string().max(50, '用户名最多 50 字').optional(),
  avatarUrl: z.string().url('请输入合法链接').or(z.literal('')).optional(),
  locale: z.string().optional()
})
type ProfileSchema = z.output<typeof profileSchema>
const profileState = reactive<ProfileSchema>({ displayName: '', username: '', avatarUrl: '', locale: '' })
watchEffect(() => {
  if (!me.value) return
  profileState.displayName = me.value.displayName
  profileState.username = me.value.username
  profileState.avatarUrl = me.value.avatarUrl
})
const savingProfile = ref(false)
async function onSaveProfile(e: FormSubmitEvent<ProfileSchema>) {
  savingProfile.value = true
  try {
    await call('/api/v1/session/profile', { method: 'PUT', body: e.data })
    await refresh()
    toast.add({ title: '资料已更新', color: 'success', icon: 'i-tabler-check' })
  } catch (err: any) {
    toast.add({ title: '保存失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    savingProfile.value = false
  }
}

// ── Change password ─────────────────────────────────────────────────────────
const pwSchema = z.object({
  currentPassword: z.string().min(1, '请输入当前密码'),
  newPassword: z.string().min(8, '新密码至少 8 位').max(128),
  confirm: z.string().min(1, '请再次输入新密码')
}).refine(d => d.newPassword === d.confirm, { message: '两次输入不一致', path: ['confirm'] })
type PwSchema = z.output<typeof pwSchema>
const pwState = reactive<Partial<PwSchema>>({ currentPassword: '', newPassword: '', confirm: '' })
const savingPw = ref(false)
async function onChangePassword(e: FormSubmitEvent<PwSchema>) {
  savingPw.value = true
  try {
    await call('/api/v1/auth/password/change', {
      method: 'POST',
      body: { currentPassword: e.data.currentPassword, newPassword: e.data.newPassword }
    })
    pwState.currentPassword = pwState.newPassword = pwState.confirm = ''
    await loadSessions()
    toast.add({ title: '密码已修改', description: '其他设备的登录会话已退出。', color: 'success', icon: 'i-tabler-lock' })
  } catch (err: any) {
    toast.add({ title: '修改失败', description: err?.data?.message || '请检查当前密码', color: 'error' })
  } finally {
    savingPw.value = false
  }
}

// ── Sessions ────────────────────────────────────────────────────────────────
interface SessionEntry {
  id: string; createdAt: string; lastSeen: string; ip: string; userAgent: string; current: boolean
}
const { data: sessionData, refresh: refreshSessions } = await useAsyncData('sessions',
  () => call<{ entries: SessionEntry[] }>('/api/v1/session/list'))
const sessions = computed(() => sessionData.value?.entries ?? [])
async function loadSessions() { await refreshSessions() }

const revoking = ref<string>('')
async function onRevoke(id: string) {
  revoking.value = id
  try {
    await call(`/api/v1/session/${id}`, { method: 'DELETE' })
    await refreshSessions()
    toast.add({ title: '已退出该会话', color: 'success' })
  } catch (err: any) {
    toast.add({ title: '操作失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    revoking.value = ''
  }
}
async function onLogoutAll() {
  await call('/api/v1/auth/logout-all', { method: 'POST' })
  await navigateTo('/login')
}
async function onLogout() {
  await logout()
  await navigateTo('/login')
}

function fmt(ts: string) {
  if (!ts) return '—'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? ts : d.toLocaleString('zh-CN')
}
function deviceIcon(ua: string) {
  return /mobile|android|iphone|ipad/i.test(ua) ? 'i-tabler-device-mobile' : 'i-tabler-device-desktop'
}
</script>

<template>
  <div class="space-y-6">
    <!-- Identity header -->
    <div class="flex items-center gap-4">
      <UAvatar :text="initial" :src="me?.avatarUrl || undefined" size="3xl" class="ring-2 ring-primary/20" />
      <div class="min-w-0">
        <h1 class="font-display truncate text-2xl font-semibold text-highlighted">{{ me?.displayName || '我的账户' }}</h1>
        <p class="truncate text-sm text-muted">{{ me?.email }}</p>
      </div>
    </div>

    <UAlert
      v-if="me && !me.emailVerified"
      color="warning" variant="soft" icon="i-tabler-mail-x"
      title="邮箱尚未验证" description="验证邮箱以保护账户安全。"
    >
      <template #actions>
        <UButton color="warning" size="sm" label="重新发送验证邮件" :loading="resending" @click="onResendVerification" />
      </template>
    </UAlert>

    <!-- Profile edit -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-tabler-user-edit" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">个人资料</h2>
        </div>
      </template>
      <UForm :schema="profileSchema" :state="profileState" class="space-y-4" @submit="onSaveProfile">
        <UFormField name="displayName" label="昵称">
          <UInput v-model="profileState.displayName" class="w-full" />
        </UFormField>
        <UFormField name="username" label="用户名" hint="可选">
          <UInput v-model="profileState.username" class="w-full" placeholder="li" />
        </UFormField>
        <UFormField name="avatarUrl" label="头像链接" hint="可选">
          <UInput v-model="profileState.avatarUrl" class="w-full" placeholder="https://…" />
        </UFormField>
        <div class="flex items-center justify-between gap-4">
          <p class="text-xs text-muted">邮箱 {{ me?.email }} 不可在此修改</p>
          <UButton type="submit" label="保存资料" :loading="savingProfile" />
        </div>
      </UForm>
    </UCard>

    <!-- Security: change password -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-tabler-lock" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">修改密码</h2>
        </div>
      </template>
      <UForm :schema="pwSchema" :state="pwState" class="space-y-4" @submit="onChangePassword">
        <UFormField name="currentPassword" label="当前密码">
          <UInput v-model="pwState.currentPassword" type="password" autocomplete="current-password" class="w-full" />
        </UFormField>
        <UFormField name="newPassword" label="新密码" hint="至少 8 位">
          <UInput v-model="pwState.newPassword" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <UFormField name="confirm" label="确认新密码">
          <UInput v-model="pwState.confirm" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <div class="flex items-center justify-between gap-4">
          <p class="text-xs text-muted">改密后其他设备会被强制登出</p>
          <UButton type="submit" color="neutral" label="修改密码" :loading="savingPw" />
        </div>
      </UForm>
    </UCard>

    <!-- Sessions -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <UIcon name="i-tabler-devices" class="size-5 text-muted" />
            <h2 class="font-medium text-highlighted">登录会话</h2>
          </div>
          <UButton
            color="error" variant="soft" size="xs" icon="i-tabler-logout-2"
            label="退出所有设备" @click="onLogoutAll"
          />
        </div>
      </template>
      <ul class="divide-y divide-default">
        <li v-for="s in sessions" :key="s.id" class="flex items-center gap-3 py-3">
          <UIcon :name="deviceIcon(s.userAgent)" class="size-5 shrink-0 text-dimmed" />
          <div class="min-w-0 flex-1">
            <p class="flex items-center gap-2 text-sm text-highlighted">
              <span class="truncate">{{ s.ip || '未知 IP' }}</span>
              <UBadge v-if="s.current" color="primary" variant="soft" size="sm">当前设备</UBadge>
            </p>
            <p class="truncate text-xs text-muted">{{ s.userAgent || '未知设备' }} · 最近活跃 {{ fmt(s.lastSeen) }}</p>
          </div>
          <UButton
            v-if="!s.current"
            color="neutral" variant="ghost" size="xs" icon="i-tabler-trash"
            :loading="revoking === s.id" aria-label="退出该会话" @click="onRevoke(s.id)"
          />
        </li>
      </ul>
    </UCard>

    <div class="flex justify-end">
      <UButton color="neutral" variant="outline" icon="i-tabler-logout" label="退出登录" @click="onLogout" />
    </div>
  </div>
</template>
