<script setup lang="ts">
import { createPlatformNotifier } from '@platform/ui/feedback'
import type { TOTPEnrollment, TOTPEntry } from '~/composables/useMFA'

const props = defineProps<{ recovery?: boolean }>()
const emit = defineEmits<{ recovered: [] }>()
const toast = createPlatformNotifier(useToast())
const mfa = useMFA()
const modalOpen = ref(false)
const removeOpen = ref(false)
const starting = ref(false)
const confirming = ref(false)
const removing = ref(false)
const stage = ref<'verify' | 'recovery'>('verify')
const enrollment = ref<TOTPEnrollment>()
const recoveryCodes = ref<string[]>([])
const code = ref('')
const selected = ref<TOTPEntry>()
const qrDataUrl = ref('')
const enrollmentExpiry = useExpiryCountdown(() => enrollment.value?.expiresAt ?? '')

const { data, pending, refresh } = await useAsyncData(
  props.recovery ? 'recovery-totp-authenticators' : 'account-totp-authenticators',
  () => props.recovery ? Promise.resolve({ entries: [] }) : mfa.listTOTP(),
  { default: () => ({ entries: [] }) },
)
const entries = computed(() => data.value.entries)

function clearEnrollment() {
  code.value = ''
  enrollment.value = undefined
  qrDataUrl.value = ''
}

function cancelEnrollment() {
  clearEnrollment()
  modalOpen.value = false
}

watch(modalOpen, (open) => {
  if (!open && stage.value === 'verify') clearEnrollment()
})

async function startEnrollment() {
  starting.value = true
  try {
    clearEnrollment()
    enrollment.value = await mfa.beginTOTP('身份验证器')
    stage.value = 'verify'
    code.value = ''
    recoveryCodes.value = []
    const qr = await $fetch<{ dataUrl: string }>('/api/totp-qr', {
      method: 'POST',
      body: { value: enrollment.value.uri },
    })
    qrDataUrl.value = qr.dataUrl
    modalOpen.value = true
  } catch (error) {
    toast.add({ title: '无法开始设置', description: identityErrorMessage(error, { context: 'mfa' }), color: 'error' })
  } finally {
    starting.value = false
  }
}

async function confirmEnrollment() {
  if (!enrollment.value || !/^\d{6}$/.test(code.value)) return
  confirming.value = true
  try {
    const result = await mfa.finishTOTP(enrollment.value.authenticatorId, code.value)
    recoveryCodes.value = result.recoveryCodes
    stage.value = 'recovery'
    await refresh()
  } catch (error) {
    toast.add({ title: '验证码未通过', description: identityErrorMessage(error, { context: 'mfa' }), color: 'error' })
  } finally {
    confirming.value = false
  }
}

async function copyRecoveryCodes() {
  try {
    await navigator.clipboard.writeText(recoveryCodes.value.join('\n'))
    // feedback-contract: Clipboard success has no durable inline state the page can observe.
    toast.add({ title: '恢复代码已复制', color: 'success' })
  } catch {
    toast.add({
      title: '无法复制恢复代码',
      description: '浏览器未授予剪贴板权限，请改用下载或手动保存。',
      color: 'error',
    })
  }
}

function downloadRecoveryCodes() {
  const text = [
    '月离账户恢复代码',
    '每个代码只能使用一次。请将它们保存在安全且与密码分离的位置。',
    '',
    ...recoveryCodes.value,
  ].join('\n')
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'yueli-recovery-codes.txt'
  anchor.click()
  URL.revokeObjectURL(url)
}

function finishRecovery() {
  recoveryCodes.value = []
  enrollment.value = undefined
  modalOpen.value = false
  if (props.recovery) emit('recovered')
}

function openRemove(entry: TOTPEntry) {
  selected.value = entry
  removeOpen.value = true
}

async function confirmRemove() {
  if (!selected.value) return
  removing.value = true
  try {
    await mfa.removeTOTP(selected.value.id)
    await refresh()
    removeOpen.value = false
  } catch (error) {
    toast.add({ title: '无法移除身份验证器', description: identityErrorMessage(error, { context: 'mfa' }), color: 'error' })
  } finally {
    removing.value = false
  }
}

function formatDate(value?: string) {
  if (!value) return '尚未使用'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN')
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="flex items-center gap-2 font-semibold text-highlighted">
            <UIcon name="i-tabler-shield-lock" class="size-5 text-primary" />
            {{ props.recovery ? '重建身份验证器' : '双重验证' }}
          </h2>
          <p class="mt-1 text-xs text-muted">
            {{ props.recovery
              ? '新验证器启用后，原有验证器和恢复代码会立即失效。'
              : '登录时使用身份验证器生成的动态口令。' }}
          </p>
        </div>
        <UButton
          icon="i-tabler-plus"
          :label="props.recovery ? '开始恢复' : '添加身份验证器'"
          :loading="starting"
          @click="startEnrollment"
        />
      </div>
    </template>

    <div v-if="pending" class="space-y-3" aria-label="正在加载身份验证器">
      <USkeleton v-for="item in 2" :key="item" class="h-16 w-full rounded-lg" />
    </div>
    <div
      v-else-if="!entries.length"
      class="rounded-lg border border-dashed border-default px-4 py-8 text-center"
    >
      <span class="mx-auto grid size-10 place-items-center rounded-full bg-elevated">
        <UIcon name="i-tabler-shield-off" class="size-5 text-muted" />
      </span>
      <p class="mt-3 text-sm font-medium text-highlighted">
        {{ props.recovery ? '需要新的身份验证器' : '尚未启用双重验证' }}
      </p>
      <p class="mt-1 text-xs text-muted">
        {{ props.recovery ? '完成设置后，请退出恢复会话并重新登录。' : '支持任意兼容 TOTP 的身份验证器应用。' }}
      </p>
    </div>
    <ul v-else class="divide-y divide-default">
      <li
        v-for="entry in entries"
        :key="entry.id"
        class="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0"
      >
        <span class="grid size-10 shrink-0 place-items-center rounded-full bg-primary/10">
          <UIcon name="i-tabler-device-mobile-code" class="size-5 text-primary" />
        </span>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-highlighted">{{ entry.label || '身份验证器' }}</p>
          <p class="mt-0.5 text-xs text-muted">
            最近使用 <ClientOnly fallback="…">{{ formatDate(entry.lastUsedAt) }}</ClientOnly>
          </p>
        </div>
        <UButton
          color="error"
          variant="ghost"
          icon="i-tabler-trash"
          label="移除"
          @click="openRemove(entry)"
        />
      </li>
    </ul>
  </UCard>

  <UModal
    v-model:open="modalOpen"
    :dismissible="false"
    title="设置身份验证器"
    :description="stage === 'verify' ? '扫描二维码并输入动态验证码。' : '保存一次性恢复代码。'"
  >
    <template #body>
      <div v-if="stage === 'verify'" class="space-y-5">
        <UAlert
          v-if="enrollmentExpiry.expired.value"
          color="error"
          variant="soft"
          icon="i-tabler-clock-x"
          title="设置已过期"
          description="二维码和密钥已经失效，请重新开始设置。"
        >
          <template #actions>
            <UButton label="重新开始" :loading="starting" @click="startEnrollment" />
          </template>
        </UAlert>
        <template v-else>
        <div class="grid gap-5 sm:grid-cols-[208px_1fr] sm:items-center">
          <div class="mx-auto overflow-hidden rounded-xl border border-default bg-white p-2">
            <img
              :src="qrDataUrl"
              width="208"
              height="208"
              class="block size-52"
              alt="身份验证器设置二维码"
            >
          </div>
          <div class="space-y-3">
            <p class="text-sm text-muted">
              用身份验证器应用扫描二维码。无法扫描时，手动输入下面的密钥。
            </p>
            <code class="block break-all rounded-lg bg-elevated px-3 py-2 text-sm text-highlighted">
              {{ enrollment?.secret }}
            </code>
          </div>
        </div>
        <p
          v-if="enrollment?.expiresAt"
          class="text-center text-xs text-muted"
          role="status"
          aria-live="polite"
        >
          设置将在 {{ enrollmentExpiry.label.value }} 后过期
        </p>
        <form class="space-y-4" @submit.prevent="confirmEnrollment">
          <UFormField label="6 位动态验证码">
            <UInput
              v-model="code"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="6"
              pattern="[0-9]{6}"
              autofocus
              class="w-full"
            />
          </UFormField>
          <div class="flex justify-end gap-2">
            <UButton
              type="button"
              color="neutral"
              variant="ghost"
              label="取消"
              :disabled="confirming"
              @click="cancelEnrollment"
            />
            <UButton
              type="submit"
              label="验证并启用"
              :disabled="!/^\d{6}$/.test(code)"
              :loading="confirming"
            />
          </div>
        </form>
        </template>
      </div>

      <div v-else class="space-y-5">
        <UAlert
          color="warning"
          variant="soft"
          icon="i-tabler-alert-triangle"
          title="恢复代码只显示这一次"
          description="每个代码只能使用一次。请保存在密码管理器或其他安全位置。"
        />
        <ul class="grid grid-cols-2 gap-2 rounded-xl bg-elevated p-4 font-mono text-sm">
          <li v-for="item in recoveryCodes" :key="item" class="select-all text-highlighted">{{ item }}</li>
        </ul>
        <div class="flex flex-wrap justify-end gap-2">
          <UButton color="neutral" variant="outline" icon="i-tabler-copy" label="复制" @click="copyRecoveryCodes" />
          <UButton color="neutral" variant="outline" icon="i-tabler-download" label="下载" @click="downloadRecoveryCodes" />
          <UButton label="我已安全保存" @click="finishRecovery" />
        </div>
      </div>
    </template>
  </UModal>

  <UModal
    v-model:open="removeOpen"
    title="移除身份验证器？"
    description="如果这是最后一个身份验证器，双重验证和现有恢复代码将同时停用。"
  >
    <template #body>
      <p class="text-sm text-muted">
        将移除「{{ selected?.label || '身份验证器' }}」。这项操作需要近期的双重身份验证。
      </p>
      <div class="mt-5 flex justify-end gap-2">
        <UButton color="neutral" variant="ghost" label="取消" @click="() => { removeOpen = false }" />
        <UButton color="error" label="确认移除" :loading="removing" @click="confirmRemove" />
      </div>
    </template>
  </UModal>
</template>
