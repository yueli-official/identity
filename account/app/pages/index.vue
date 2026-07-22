<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import { SOCIAL_PLATFORMS, socialPlatform } from '@platform/ui/social'
import { createPlatformNotifier } from '@platform/ui/feedback'
import { useActionFeedback } from '@yueli/ui/feedback'
import { ActionFeedbackButton } from '@yueli/ui/feedback/pattern'
import type { SocialLink } from '~/composables/useSession'

definePageMeta({ middleware: 'auth' })

const { me, refresh, logout } = useSession()
const { call } = useApi()
const toast = createPlatformNotifier(useToast())

const initial = computed(() =>
  (me.value?.displayName || me.value?.email || '?').charAt(0).toUpperCase()
)

// ── Email verification ──────────────────────────────────────────────────────
const resending = ref(false)
async function onResendVerification() {
  resending.value = true
  try {
    await call('/api/v1/auth/email/verify-request', { method: 'POST' })
    // feedback-contract: the email is delivered outside the current surface
    toast.add({ title: '验证邮件已发送', description: '请检查邮箱完成验证。(demo 无真实邮件:链接打印在后端日志)', color: 'success', icon: 'i-tabler-mail-check' })
  } catch (err: any) {
    toast.add({ title: '发送失败', description: err?.data?.message || '请稍后重试。', color: 'error' })
  } finally {
    resending.value = false
  }
}

// ── Profile edit ────────────────────────────────────────────────────────────
// avatar/cover are committed immediately by their crop uploaders; the form below
// saves the text fields + social links.
const profileSchema = z.object({
  displayName: z.string().min(1, '请输入昵称'),
  username: z.string().max(50, '用户名最多 50 字').optional(),
  bio: z.string().max(500, '简介最多 500 字').optional(),
  locale: z.string().optional()
})
type ProfileSchema = z.output<typeof profileSchema>
const profileState = reactive<ProfileSchema>({ displayName: '', username: '', bio: '', locale: '' })
const avatarUrl = ref('')
const coverUrl = ref('')

// social links: edited as {platform-key, url} rows; persisted as {label, url}.
const platformItems = SOCIAL_PLATFORMS.map(p => ({ label: p.label, value: p.key, icon: p.icon }))
const plat = (key: string) => SOCIAL_PLATFORMS.find(p => p.key === key) ?? SOCIAL_PLATFORMS[SOCIAL_PLATFORMS.length - 1]!
interface SocialRow { key: string, url: string }
const socialRows = ref<SocialRow[]>([])

watchEffect(() => {
  if (!me.value) return
  profileState.displayName = me.value.displayName
  profileState.username = me.value.username
  profileState.bio = me.value.bio
  avatarUrl.value = me.value.avatarUrl
  coverUrl.value = me.value.coverUrl
  socialRows.value = (me.value.socialLinks ?? []).map(l => ({ key: socialPlatform(l).key, url: l.url }))
})

function addLink() {
  const used = new Set(socialRows.value.map(r => r.key))
  const next = SOCIAL_PLATFORMS.find(p => !used.has(p.key)) ?? SOCIAL_PLATFORMS[0]!
  socialRows.value.push({ key: next.key, url: '' })
}
function removeLink(i: number) { socialRows.value.splice(i, 1) }

const { status: profileSaveStatus, pending: markProfileSaving, success: markProfileSaved, error: markProfileError } = useActionFeedback()
async function onSaveProfile(e: FormSubmitEvent<ProfileSchema>) {
  markProfileSaving()
  try {
    const socialLinks: SocialLink[] = socialRows.value
      .filter(r => r.url.trim())
      .map(r => ({ label: plat(r.key).label, url: r.url.trim() }))
    await call('/api/v1/session/profile', { method: 'PUT', body: {
      ...e.data, avatarUrl: avatarUrl.value, coverUrl: coverUrl.value, socialLinks
    } })
    await refresh()
    markProfileSaved()
  } catch (err: any) {
    markProfileError()
    toast.add({ title: '保存失败', description: err?.data?.message || '请重试', color: 'error' })
  }
}
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
const { status: passwordSaveStatus, pending: markPasswordSaving, success: markPasswordSaved, error: markPasswordError } = useActionFeedback()
async function onChangePassword(e: FormSubmitEvent<PwSchema>) {
  markPasswordSaving()
  try {
    await call('/api/v1/auth/password/change', {
      method: 'POST',
      body: { currentPassword: e.data.currentPassword, newPassword: e.data.newPassword }
    })
    pwState.currentPassword = pwState.newPassword = pwState.confirm = ''
    await loadSessions()
    markPasswordSaved()
  } catch (err: any) {
    markPasswordError()
    toast.add({ title: '修改失败', description: err?.data?.message || '请检查当前密码', color: 'error' })
  }
}

// ── Set initial password (accounts that have none, e.g. OAuth-only) ──────────
const setPwSchema = z.object({
  newPassword: z.string().min(8, '密码至少 8 位').max(128),
  confirm: z.string().min(1, '请再次输入密码')
}).refine(d => d.newPassword === d.confirm, { message: '两次输入不一致', path: ['confirm'] })
type SetPwSchema = z.output<typeof setPwSchema>
const setPwState = reactive<Partial<SetPwSchema>>({ newPassword: '', confirm: '' })
const { status: initialPasswordStatus, pending: markInitialPasswordSaving, success: markInitialPasswordSaved, error: markInitialPasswordError } = useActionFeedback()
async function onSetPassword(e: FormSubmitEvent<SetPwSchema>) {
  markInitialPasswordSaving()
  try {
    await call('/api/v1/auth/password/set', { method: 'POST', body: { newPassword: e.data.newPassword } })
    setPwState.newPassword = setPwState.confirm = ''
    await refreshCreds()
    markInitialPasswordSaved()
  } catch (err: any) {
    markInitialPasswordError()
    toast.add({ title: '设置失败', description: err?.data?.message || '请重试', color: 'error' })
  }
}

// ── Sessions ────────────────────────────────────────────────────────────────
interface SessionEntry {
  id: string; createdAt: string; lastSeen: string; ip: string; userAgent: string; current: boolean
}
const { data: sessionData, refresh: refreshSessions } = await useAsyncData('sessions',
  () => call<{ entries: SessionEntry[] }>('/api/v1/session/list'))
const sessions = computed(() => sessionData.value?.entries ?? [])
const currentSession = computed(() => sessions.value.find(s => s.current))
const otherSessions = computed(() => sessions.value.filter(s => !s.current))
const showAllSessions = ref(false)
const visibleOthers = computed(() => showAllSessions.value ? otherSessions.value : otherSessions.value.slice(0, 3))
async function loadSessions() { await refreshSessions() }

const revoking = ref<string>('')
async function onRevoke(id: string) {
  revoking.value = id
  try {
    await call(`/api/v1/session/${id}`, { method: 'DELETE' })
    await refreshSessions()
  } catch (err: any) {
    toast.add({ title: '操作失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    revoking.value = ''
  }
}
const loggingOutAll = ref(false)
async function onLogoutAll() {
  loggingOutAll.value = true
  try {
    await call('/api/v1/auth/logout-all', { method: 'POST' })
    await navigateTo('/login')
  } finally {
    loggingOutAll.value = false
  }
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
  } catch (err: any) {
    toast.add({ title: '解绑失败', description: err?.data?.message || '请重试', color: 'error' })
  } finally {
    unbinding.value = false
  }
}
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
function shortUA(ua: string) {
  if (!ua) return '未知设备'
  const browser = /edg/i.test(ua) ? 'Edge' : /chrome/i.test(ua) ? 'Chrome' : /firefox/i.test(ua) ? 'Firefox' : /safari/i.test(ua) ? 'Safari' : '浏览器'
  const os = /windows/i.test(ua) ? 'Windows' : /mac/i.test(ua) ? 'macOS' : /android/i.test(ua) ? 'Android' : /iphone|ipad/i.test(ua) ? 'iOS' : /linux/i.test(ua) ? 'Linux' : ''
  return os ? `${browser} · ${os}` : browser
}

// section header helper
const cardHeaderClass = 'flex items-center gap-2 font-semibold text-highlighted'
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-6">
    <!-- Identity header: cover banner + overlapping avatar (both crop-uploadable) -->
    <section class="overflow-hidden rounded-xl ring-1 ring-default bg-default shadow-sm">
      <UserCoverCrop :model-value="coverUrl" editable @update:model-value="onCoverUpdated" />
      <div class="flex items-end gap-4 px-5 pb-5">
        <div class="-mt-12 shrink-0 rounded-full ring-4 ring-default">
          <UserAvatarCrop :model-value="avatarUrl" :initial="initial" editable @update:model-value="onAvatarUpdated" />
        </div>
        <div class="min-w-0 flex-1 pb-1">
          <h1 class="font-display flex items-center gap-2 truncate text-xl font-semibold text-highlighted">
            {{ me?.displayName || '我的账户' }}
            <UBadge v-if="me?.emailVerified" color="success" variant="subtle" size="sm" icon="i-tabler-rosette-discount-check">已验证</UBadge>
          </h1>
          <p class="truncate text-sm text-muted">{{ me?.email }}</p>
        </div>
      </div>
    </section>

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
    <UCard id="profile-settings" class="scroll-mt-24">
      <template #header>
        <h2 :class="cardHeaderClass"><UIcon name="i-tabler-user-edit" class="size-5 text-primary" />个人资料</h2>
      </template>
      <UForm :schema="profileSchema" :state="profileState" class="space-y-4" @submit="onSaveProfile">
        <UFormField name="displayName" label="昵称">
          <UInput v-model="profileState.displayName" class="w-full" placeholder="你的昵称" />
        </UFormField>
        <UFormField name="username" label="用户名" hint="可选,用于个性化地址">
          <UInput v-model="profileState.username" class="w-full" placeholder="li" />
        </UFormField>
        <UFormField name="bio" label="简介" hint="可选">
          <UTextarea v-model="profileState.bio" :rows="3" class="w-full" placeholder="一句话介绍自己" />
        </UFormField>

        <!-- social links: preset platform picker + url -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <p class="text-sm font-medium text-default">社交链接</p>
            <UButton label="添加链接" icon="i-tabler-plus" color="neutral" variant="soft" size="xs"
              :disabled="socialRows.length >= platformItems.length" @click="addLink" />
          </div>
          <div v-if="!socialRows.length" class="rounded-md border border-dashed border-default px-3 py-5 text-center text-xs text-dimmed">
            还没有链接 —— 点「添加链接」从预设平台中选择
          </div>
          <div v-else class="space-y-2">
            <div v-for="(row, i) in socialRows" :key="i" class="flex items-center gap-2">
              <USelectMenu
                v-model="row.key" :items="platformItems" value-key="value"
                :icon="plat(row.key).icon" class="w-36 shrink-0" :search-input="false"
              />
              <UInput v-model="row.url" :placeholder="plat(row.key).placeholder" class="min-w-0 flex-1" />
              <UButton icon="i-tabler-trash" color="error" variant="ghost" size="sm" square aria-label="删除" @click="removeLink(i)" />
            </div>
          </div>
        </div>

        <div class="flex items-center justify-between gap-4 border-t border-default pt-4">
          <p class="text-xs text-muted">邮箱 {{ me?.email }} 不可在此修改</p>
          <ActionFeedbackButton
            type="submit"
            :status="profileSaveStatus"
            idle-label="保存资料"
            pending-label="保存中"
            success-label="已保存"
            error-label="保存失败"
          />
        </div>
      </UForm>
    </UCard>

    <!-- Security: change password (only when a password is already set) -->
    <UCard v-if="credData?.hasPassword">
      <template #header>
        <h2 :class="cardHeaderClass"><UIcon name="i-tabler-lock" class="size-5 text-primary" />修改密码</h2>
      </template>
      <UForm :schema="pwSchema" :state="pwState" class="space-y-4" @submit="onChangePassword">
        <UFormField name="currentPassword" label="当前密码">
          <UInput v-model="pwState.currentPassword" type="password" autocomplete="current-password" class="w-full" />
        </UFormField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField name="newPassword" label="新密码" hint="至少 8 位">
            <UInput v-model="pwState.newPassword" type="password" autocomplete="new-password" class="w-full" />
          </UFormField>
          <UFormField name="confirm" label="确认新密码">
            <UInput v-model="pwState.confirm" type="password" autocomplete="new-password" class="w-full" />
          </UFormField>
        </div>
        <div class="flex items-center justify-between gap-4 border-t border-default pt-4">
          <p class="text-xs text-muted">改密后其他设备会被强制登出</p>
          <ActionFeedbackButton
            type="submit"
            color="neutral"
            :status="passwordSaveStatus"
            idle-label="修改密码"
            pending-label="修改中"
            success-label="已修改"
            error-label="修改失败"
          />
        </div>
      </UForm>
    </UCard>

    <!-- Security: set an initial password (accounts without one, e.g. OAuth-only) -->
    <UCard v-else>
      <template #header>
        <h2 :class="cardHeaderClass"><UIcon name="i-tabler-lock-plus" class="size-5 text-primary" />设置密码</h2>
      </template>
      <UForm :schema="setPwSchema" :state="setPwState" class="space-y-4" @submit="onSetPassword">
        <UAlert
          color="info" variant="soft" icon="i-tabler-info-circle"
          title="你还没有设置密码"
          description="你目前只能用第三方账号登录。设置密码后即可用邮箱 + 密码登录,也才能解绑第三方账号。"
        />
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField name="newPassword" label="新密码" hint="至少 8 位">
            <UInput v-model="setPwState.newPassword" type="password" autocomplete="new-password" class="w-full" />
          </UFormField>
          <UFormField name="confirm" label="确认密码">
            <UInput v-model="setPwState.confirm" type="password" autocomplete="new-password" class="w-full" />
          </UFormField>
        </div>
        <div class="flex justify-end">
          <ActionFeedbackButton
            type="submit"
            :status="initialPasswordStatus"
            idle-label="设置密码"
            pending-label="设置中"
            success-label="已设置"
            error-label="设置失败"
          />
        </div>
      </UForm>
    </UCard>

    <!-- Credentials / login methods -->
    <UCard>
      <template #header>
        <h2 :class="cardHeaderClass"><UIcon name="i-tabler-key" class="size-5 text-primary" />登录方式</h2>
      </template>
      <ul class="divide-y divide-default">
        <li class="flex items-center justify-between gap-4 py-3 first:pt-0">
          <div class="flex items-center gap-3">
            <div class="grid size-9 shrink-0 place-items-center rounded-full bg-elevated">
              <UIcon name="i-tabler-lock" class="size-4.5 text-muted" />
            </div>
            <div>
              <p class="text-sm font-medium text-highlighted">密码</p>
              <p class="text-xs text-muted">邮箱 + 密码登录</p>
            </div>
          </div>
          <UBadge :color="credData?.hasPassword ? 'success' : 'neutral'" variant="soft" size="sm">
            {{ credData?.hasPassword ? '已设置' : '未设置' }}
          </UBadge>
        </li>
        <li class="flex items-center justify-between gap-4 py-3 last:pb-0">
          <div class="flex items-center gap-3">
            <div class="grid size-9 shrink-0 place-items-center rounded-full bg-elevated">
              <UIcon name="i-tabler-brand-google" class="size-4.5 text-muted" />
            </div>
            <div>
              <p class="text-sm font-medium text-highlighted">Google</p>
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
            @click="() => { confirmUnbindOpen = true }"
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
          <UButton color="neutral" variant="ghost" label="取消" :disabled="unbinding" @click="() => { confirmUnbindOpen = false }" />
          <UButton color="error" label="确认解绑" :loading="unbinding" @click="onUnbindGoogle" />
        </div>
      </template>
    </UModal>

    <!-- Sessions: current device highlighted, others collapsed -->
    <UCard>
      <template #header>
        <div class="flex items-center justify-between gap-2">
          <h2 :class="cardHeaderClass"><UIcon name="i-tabler-devices" class="size-5 text-primary" />登录会话</h2>
          <UButton
            v-if="otherSessions.length"
            color="error" variant="soft" size="xs" icon="i-tabler-logout-2"
            label="退出其他设备" :loading="loggingOutAll" @click="onLogoutAll"
          />
        </div>
      </template>

      <!-- current device -->
      <div v-if="currentSession" class="flex items-center gap-3 rounded-md bg-primary/5 ring-1 ring-primary/15 px-3 py-3">
        <div class="grid size-9 shrink-0 place-items-center rounded-full bg-primary/10">
          <UIcon :name="deviceIcon(currentSession.userAgent)" class="size-4.5 text-primary" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="flex items-center gap-2 text-sm font-medium text-highlighted">
            <span class="truncate">{{ shortUA(currentSession.userAgent) }}</span>
            <UBadge color="primary" variant="soft" size="sm">当前设备</UBadge>
          </p>
          <p class="truncate text-xs text-muted">
            {{ currentSession.ip || '未知 IP' }} · 最近活跃 <ClientOnly fallback="…">{{ fmt(currentSession.lastSeen) }}</ClientOnly>
          </p>
        </div>
      </div>

      <!-- other devices -->
      <div v-if="otherSessions.length" class="mt-2">
        <p class="px-1 pb-1 pt-2 text-xs font-medium text-muted">其他设备 ({{ otherSessions.length }})</p>
        <ul class="divide-y divide-default">
          <li v-for="s in visibleOthers" :key="s.id" class="flex items-center gap-3 py-3">
            <div class="grid size-9 shrink-0 place-items-center rounded-full bg-elevated">
              <UIcon :name="deviceIcon(s.userAgent)" class="size-4.5 text-muted" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm text-highlighted">{{ shortUA(s.userAgent) }}</p>
              <p class="truncate text-xs text-muted">
                {{ s.ip || '未知 IP' }} · <ClientOnly fallback="…">{{ fmt(s.lastSeen) }}</ClientOnly>
              </p>
            </div>
            <UButton
              color="neutral" variant="ghost" size="xs" icon="i-tabler-x" square
              :loading="revoking === s.id" aria-label="退出该会话" @click="onRevoke(s.id)"
            />
          </li>
        </ul>
        <UButton
          v-if="otherSessions.length > 3"
          :label="showAllSessions ? '收起' : `显示全部 ${otherSessions.length} 个会话`"
          :icon="showAllSessions ? 'i-tabler-chevron-up' : 'i-tabler-chevron-down'"
          color="neutral" variant="ghost" size="xs" block class="mt-1"
          @click="() => { showAllSessions = !showAllSessions }"
        />
      </div>
    </UCard>

    <div class="flex justify-end pb-4">
      <UButton color="neutral" variant="outline" icon="i-tabler-logout" label="退出登录" @click="onLogout" />
    </div>
  </div>
</template>
