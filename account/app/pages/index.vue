<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import type { SocialLink } from '~/composables/useSession'

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
// avatar/cover are committed immediately by their crop uploaders (the IdP proxy
// stores them on save); the form below saves the text fields + social links.
const profileSchema = z.object({
  displayName: z.string().min(1, '请输入昵称'),
  username: z.string().max(50, '用户名最多 50 字').optional(),
  bio: z.string().max(500, '简介最多 500 字').optional(),
  locale: z.string().optional()
})
type ProfileSchema = z.output<typeof profileSchema>
const profileState = reactive<ProfileSchema>({ displayName: '', username: '', bio: '', locale: '' })
// avatar/cover live outside the validated form (managed by the crop uploaders).
const avatarUrl = ref('')
const coverUrl = ref('')
const socialLinks = ref<SocialLink[]>([])
watchEffect(() => {
  if (!me.value) return
  profileState.displayName = me.value.displayName
  profileState.username = me.value.username
  profileState.bio = me.value.bio
  avatarUrl.value = me.value.avatarUrl
  coverUrl.value = me.value.coverUrl
  socialLinks.value = me.value.socialLinks?.length ? me.value.socialLinks.map(l => ({ ...l })) : []
})
function addLink() { socialLinks.value.push({ label: '', url: '' }) }
function removeLink(i: number) { socialLinks.value.splice(i, 1) }
const savingProfile = ref(false)
async function onSaveProfile(e: FormSubmitEvent<ProfileSchema>) {
  savingProfile.value = true
  try {
    await call('/api/v1/session/profile', { method: 'PUT', body: {
      ...e.data,
      avatarUrl: avatarUrl.value,
      coverUrl: coverUrl.value,
      socialLinks: socialLinks.value.filter(l => l.url.trim())
    } })
    await refresh()
    toast.add({ title: '资料已更新', color: 'success', icon: 'i-tabler-check' })
  } catch (err: any) {
    toast.add({ title: '保存失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    savingProfile.value = false
  }
}
// crop uploaders persist immediately; mirror the new URL locally + re-sync me.
async function onAvatarUpdated(url: string) { avatarUrl.value = url; await refresh() }
async function onCoverUpdated(url: string) { coverUrl.value = url; await refresh() }

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

// ── Set initial password (accounts that have none, e.g. OAuth-only) ──────────
const setPwSchema = z.object({
  newPassword: z.string().min(8, '密码至少 8 位').max(128),
  confirm: z.string().min(1, '请再次输入密码')
}).refine(d => d.newPassword === d.confirm, { message: '两次输入不一致', path: ['confirm'] })
type SetPwSchema = z.output<typeof setPwSchema>
const setPwState = reactive<Partial<SetPwSchema>>({ newPassword: '', confirm: '' })
const settingPw = ref(false)
async function onSetPassword(e: FormSubmitEvent<SetPwSchema>) {
  settingPw.value = true
  try {
    await call('/api/v1/auth/password/set', { method: 'POST', body: { newPassword: e.data.newPassword } })
    setPwState.newPassword = setPwState.confirm = ''
    await refreshCreds()
    toast.add({ title: '密码已设置', description: '现在可以用邮箱 + 密码登录,也能解绑第三方账号了。', color: 'success', icon: 'i-tabler-lock-check' })
  } catch (err: any) {
    toast.add({ title: '设置失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    settingPw.value = false
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

// ── Credentials (login methods) ─────────────────────────────────────────────
interface CredRes { hasPassword: boolean; oauth: { provider: string; email: string }[] }
const { data: credData, refresh: refreshCreds } = await useAsyncData('credentials',
  () => call<CredRes>('/api/v1/session/credentials'))
const hasGoogle = computed(() => (credData.value?.oauth ?? []).some(o => o.provider === 'google'))
const bindGoogleUrl = '/api/v1/auth/oauth/google/start?intent=bind&return_to=' + encodeURIComponent('/')
// Google is the LAST login credential when there's no password and it's the only
// oauth link. The backend refuses to unbind it (anti-lockout) — so disable the
// button and steer the user to set a password first, instead of failing on click.
const isGoogleLastCredential = computed(() =>
  !credData.value?.hasPassword && (credData.value?.oauth?.length ?? 0) <= 1
)
const confirmUnbindOpen = ref(false)
const unbinding = ref(false)
async function onUnbindGoogle() {
  unbinding.value = true
  try {
    await call('/api/v1/session/credentials/google', { method: 'DELETE' })
    await refreshCreds()
    confirmUnbindOpen.value = false
    toast.add({ title: '已解绑 Google', color: 'success', icon: 'i-tabler-check' })
  } catch (err: any) {
    toast.add({ title: '解绑失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    unbinding.value = false
  }
}
// Surface a failed Google bind handed back via ?error on the callback redirect.
const route = useRoute()
onMounted(() => {
  if (route.query.error === 'oauth_bind') {
    toast.add({ title: 'Google 绑定失败', description: '该 Google 账号可能已绑定到其它账户。', color: 'error' })
  }
})

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
    <!-- Identity header: cover banner + overlapping avatar (both crop-uploadable) -->
    <div>
      <UserCoverCrop :model-value="coverUrl" editable @update:model-value="onCoverUpdated" />
      <div class="-mt-12 flex items-end gap-4 px-4">
        <UserAvatarCrop :model-value="avatarUrl" :initial="initial" editable @update:model-value="onAvatarUpdated" />
        <div class="min-w-0 pb-1">
          <h1 class="font-display truncate text-2xl font-semibold text-highlighted">{{ me?.displayName || '我的账户' }}</h1>
          <p class="truncate text-sm text-muted">{{ me?.email }}</p>
        </div>
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
        <UFormField name="bio" label="简介" hint="可选">
          <UTextarea v-model="profileState.bio" :rows="3" class="w-full" placeholder="一句话介绍自己" />
        </UFormField>
        <!-- social links -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <p class="text-sm font-medium text-default">社交链接</p>
            <UButton label="添加" icon="i-tabler-plus" color="neutral" variant="soft" size="xs" @click="addLink" />
          </div>
          <div v-if="!socialLinks.length" class="rounded-lg border border-dashed border-default px-3 py-4 text-center text-xs text-dimmed">还没有链接</div>
          <div v-for="(link, i) in socialLinks" :key="i" class="mb-2 flex items-center gap-2">
            <UInput v-model="link.label" placeholder="标签 (GitHub)" class="w-36" />
            <UInput v-model="link.url" placeholder="https://…" class="flex-1" />
            <UButton icon="i-tabler-trash" color="error" variant="ghost" size="sm" square aria-label="删除" @click="removeLink(i)" />
          </div>
        </div>
        <div class="flex items-center justify-between gap-4">
          <p class="text-xs text-muted">邮箱 {{ me?.email }} 不可在此修改</p>
          <UButton type="submit" label="保存资料" :loading="savingProfile" />
        </div>
      </UForm>
    </UCard>

    <!-- Security: change password (only when a password is already set) -->
    <UCard v-if="credData?.hasPassword" class="shadow-soft">
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

    <!-- Security: set an initial password (accounts without one, e.g. OAuth-only) -->
    <UCard v-else class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-tabler-lock-plus" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">设置密码</h2>
        </div>
      </template>
      <UForm :schema="setPwSchema" :state="setPwState" class="space-y-4" @submit="onSetPassword">
        <UAlert
          color="info" variant="soft" icon="i-tabler-info-circle" class="mb-1"
          title="你还没有设置密码"
          description="你目前只能用第三方账号登录。设置密码后即可用邮箱 + 密码登录,也才能解绑第三方账号。"
        />
        <UFormField name="newPassword" label="新密码" hint="至少 8 位">
          <UInput v-model="setPwState.newPassword" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <UFormField name="confirm" label="确认密码">
          <UInput v-model="setPwState.confirm" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <div class="flex justify-end">
          <UButton type="submit" label="设置密码" :loading="settingPw" />
        </div>
      </UForm>
    </UCard>

    <!-- Credentials / login methods -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-tabler-key" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">登录方式</h2>
        </div>
      </template>
      <ul class="divide-y divide-default">
        <li class="flex items-center justify-between gap-4 py-3">
          <div class="flex items-center gap-3">
            <UIcon name="i-tabler-lock" class="size-5 text-dimmed" />
            <div>
              <p class="text-sm text-highlighted">密码</p>
              <p class="text-xs text-muted">邮箱 + 密码登录</p>
            </div>
          </div>
          <UBadge :color="credData?.hasPassword ? 'success' : 'neutral'" variant="soft" size="sm">
            {{ credData?.hasPassword ? '已设置' : '未设置' }}
          </UBadge>
        </li>
        <li class="flex items-center justify-between gap-4 py-3">
          <div class="flex items-center gap-3">
            <UIcon name="i-tabler-brand-google" class="size-5 text-dimmed" />
            <div>
              <p class="text-sm text-highlighted">Google</p>
              <p class="text-xs text-muted">
                {{ hasGoogle ? '已绑定' : '未绑定' }}
                <span v-if="hasGoogle && isGoogleLastCredential" class="text-warning">· 唯一登录方式,需先设置密码才能解绑</span>
              </p>
            </div>
          </div>
          <UButton
            v-if="hasGoogle"
            color="neutral" variant="ghost" size="xs" label="解绑"
            :disabled="isGoogleLastCredential || unbinding"
            @click="confirmUnbindOpen = true"
          />
          <UButton
            v-else
            color="neutral" variant="outline" size="xs" icon="i-tabler-brand-google"
            label="绑定" :to="bindGoogleUrl" external
          />
        </li>
      </ul>
    </UCard>

    <!-- Confirm unbind (destructive, so gate it behind an explicit confirmation) -->
    <UModal v-model:open="confirmUnbindOpen" title="解绑 Google?">
      <template #body>
        <p class="text-sm text-muted">
          解绑后将无法再用 Google 登录此账户,你仍可用邮箱 + 密码登录。确定要解绑吗?
        </p>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" label="取消" :disabled="unbinding" @click="confirmUnbindOpen = false" />
          <UButton color="error" label="确认解绑" :loading="unbinding" @click="onUnbindGoogle" />
        </div>
      </template>
    </UModal>

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
            <p class="truncate text-xs text-muted">
              {{ s.userAgent || '未知设备' }} · 最近活跃
              <!-- toLocaleString is timezone/locale dependent → render client-only so
                   the SSR and client markup agree (a mismatch here was eating the
                   page's first click after a hard load / OAuth redirect). -->
              <ClientOnly fallback="…">{{ fmt(s.lastSeen) }}</ClientOnly>
            </p>
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
