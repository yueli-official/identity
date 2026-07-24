<script setup lang="ts">
import { createPlatformNotifier } from '@platform/ui/feedback'
import type { PasskeyEntry } from '~/composables/usePasskeys'
import { passkeyErrorMessage } from '~/composables/usePasskeys'

const emit = defineEmits<{ changed: [] }>()
const toast = createPlatformNotifier(useToast())
const passkeys = usePasskeys()
const browserSupported = ref(false)
const adding = ref(false)
const renaming = ref(false)
const removing = ref(false)
const renameOpen = ref(false)
const removeOpen = ref(false)
const selected = ref<PasskeyEntry>()
const renameLabel = ref('')

const { data, pending, refresh } = await useAsyncData(
  'account-passkeys',
  passkeys.list,
  { default: () => ({ entries: [] }) },
)
const entries = computed(() => data.value.entries)

onMounted(() => {
  browserSupported.value = passkeys.isSupported()
})

function defaultLabel() {
  const mobile = /mobile|android|iphone|ipad/i.test(navigator.userAgent)
  return `${mobile ? '移动设备' : '电脑'} · ${new Date().toLocaleDateString('zh-CN')}`
}

async function addPasskey() {
  adding.value = true
  try {
    await passkeys.register(defaultLabel())
    await refresh()
    emit('changed')
    toast.add({
      title: '通行密钥已添加',
      description: '现在可以使用设备解锁方式登录。',
      color: 'success',
      icon: 'i-tabler-key',
    })
  } catch (error) {
    toast.add({ title: '未能添加通行密钥', description: passkeyErrorMessage(error), color: 'error' })
  } finally {
    adding.value = false
  }
}

function cancelAddPasskey() {
  passkeys.cancelCeremony()
}

function openRename(entry: PasskeyEntry) {
  selected.value = entry
  renameLabel.value = entry.label
  renameOpen.value = true
}

async function submitRename() {
  if (!selected.value) return
  renaming.value = true
  try {
    await passkeys.rename(selected.value.id, renameLabel.value.trim())
    await refresh()
    renameOpen.value = false
    toast.add({ title: '名称已更新', color: 'success' })
  } catch (error) {
    toast.add({ title: '重命名失败', description: passkeyErrorMessage(error), color: 'error' })
  } finally {
    renaming.value = false
  }
}

function openRemove(entry: PasskeyEntry) {
  selected.value = entry
  removeOpen.value = true
}

async function confirmRemove() {
  if (!selected.value) return
  removing.value = true
  try {
    await passkeys.remove(selected.value.id)
    await refresh()
    emit('changed')
    removeOpen.value = false
    toast.add({ title: '通行密钥已移除', color: 'success' })
  } catch (error) {
    toast.add({
      title: '无法移除通行密钥',
      description: passkeyErrorMessage(error),
      color: 'error',
    })
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
            <UIcon name="i-tabler-key" class="size-5 text-primary" />
            通行密钥
          </h2>
          <p class="mt-1 text-xs text-muted">使用指纹、面容或设备 PIN 登录，无需输入密码。</p>
        </div>
        <UButton
          v-if="adding"
          color="neutral"
          variant="outline"
          icon="i-tabler-x"
          label="取消添加"
          @click="cancelAddPasskey"
        />
        <UButton
          v-else
          icon="i-tabler-plus"
          label="添加通行密钥"
          :disabled="!browserSupported"
          @click="addPasskey"
        />
      </div>
    </template>

    <UAlert
      v-if="!browserSupported"
      color="neutral"
      variant="soft"
      icon="i-tabler-browser-off"
      title="当前浏览器不支持通行密钥"
      description="你仍可查看和移除已有凭据；请使用较新的浏览器添加通行密钥。"
      class="mb-4"
    />

    <div v-if="pending" class="space-y-3" aria-label="正在加载通行密钥">
      <USkeleton v-for="item in 2" :key="item" class="h-16 w-full rounded-lg" />
    </div>

    <div
      v-else-if="!entries.length"
      class="rounded-lg border border-dashed border-default px-4 py-8 text-center"
    >
      <span class="mx-auto grid size-10 place-items-center rounded-full bg-elevated">
        <UIcon name="i-tabler-key-off" class="size-5 text-muted" />
      </span>
      <p class="mt-3 text-sm font-medium text-highlighted">还没有通行密钥</p>
      <p class="mt-1 text-xs text-muted">添加后即可在支持的设备上快速、安全地登录。</p>
    </div>

    <ul v-else class="divide-y divide-default">
      <li
        v-for="entry in entries"
        :key="entry.id"
        class="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0"
      >
        <span class="grid size-10 shrink-0 place-items-center rounded-full bg-primary/10">
          <UIcon
            :name="entry.attachment === 'platform' ? 'i-tabler-device-mobile-check' : 'i-tabler-key'"
            class="size-5 text-primary"
          />
        </span>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-highlighted">{{ entry.label || '未命名通行密钥' }}</p>
          <p class="mt-0.5 text-xs text-muted">
            {{ entry.backupEligible ? '可跨设备同步' : '仅保存在验证器中' }}
            · 最近使用 <ClientOnly fallback="…">{{ formatDate(entry.lastUsedAt) }}</ClientOnly>
          </p>
        </div>
        <div class="flex min-h-11 items-center gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            icon="i-tabler-pencil"
            label="重命名"
            @click="openRename(entry)"
          />
          <UButton
            color="error"
            variant="ghost"
            icon="i-tabler-trash"
            label="移除"
            @click="openRemove(entry)"
          />
        </div>
      </li>
    </ul>
  </UCard>

  <UModal v-model:open="renameOpen" title="重命名通行密钥" description="使用容易辨认的设备名称。">
    <template #body>
      <form class="space-y-4" @submit.prevent="submitRename">
        <UFormField label="名称" hint="最多 100 个字符">
          <UInput v-model="renameLabel" maxlength="100" autofocus class="w-full" />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="renaming"
            @click="() => { renameOpen = false }"
          />
          <UButton type="submit" label="保存" :loading="renaming" />
        </div>
      </form>
    </template>
  </UModal>

  <UModal
    v-model:open="removeOpen"
    title="移除通行密钥？"
    description="这项操作不会删除设备本地保存的凭据，但服务器将不再接受它。"
  >
    <template #body>
      <p class="text-sm text-muted">
        将移除「{{ selected?.label || '未命名通行密钥' }}」。为避免账户被锁定，至少需要保留一种其他登录方式。
      </p>
      <div class="mt-5 flex justify-end gap-2">
        <UButton
          color="neutral"
          variant="ghost"
          label="取消"
          :disabled="removing"
          @click="() => { removeOpen = false }"
        />
        <UButton color="error" label="确认移除" :loading="removing" @click="confirmRemove" />
      </div>
    </template>
  </UModal>
</template>
