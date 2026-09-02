<script setup lang="ts">
import { ManageEmpty } from '~/components/manage'
import type {
  AssetStorageBackendDetail,
  AssetStorageBackendEvent,
  AssetStorageBackendForm
} from '~/types/asset-admin'
import {
  cosEndpointForRegion,
  normalizeStorageBackendForSubmit,
  storageBackendDefaultsForType,
} from '~/utils/asset-storage-backend'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetStorageBackendForm
  editingName?: string
  detail?: AssetStorageBackendDetail | null
  events?: readonly AssetStorageBackendEvent[]
  typeOptions: SelectOption[]
  loading?: boolean
  saving?: boolean
  checking?: boolean
}>(), {
  editingName: '',
  detail: null,
  events: () => [],
  loading: false,
  saving: false,
  checking: false
})

const emit = defineEmits<{
  save: [value: AssetStorageBackendForm]
  checkHealth: []
  refreshEvents: []
  rotateSecret: []
  delete: []
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetStorageBackendForm>({ ...props.initialValue })
const isCOS = computed(() => form.type === 'cos')
const cosBucket = computed({
  get: () => form.bucketPublic || form.bucketPrivate,
  set: (value: string) => {
    form.bucketPublic = value
    form.bucketPrivate = value
  }
})
const cosEndpoint = computed(() => cosEndpointForRegion(form.region))

const canSave = computed(() => Boolean(
  form.name.trim()
  && form.type.trim()
  && form.region.trim()
  && (isCOS.value
    ? cosBucket.value.trim()
    : form.endpoint.trim() && form.bucketPublic.trim() && form.bucketPrivate.trim())
  && form.accessKey.trim()
  && (props.editingName || form.secretKey.trim())
))

watch(open, (isOpen) => {
  if (isOpen) Object.assign(form, props.initialValue)
})

watch(() => props.initialValue, (value) => {
  if (open.value) Object.assign(form, value)
}, { deep: true })

watch(() => form.type, (type, previous) => {
  if (!open.value || props.editingName || type === previous) return
  Object.assign(form, storageBackendDefaultsForType(type))
}, { flush: 'sync' })

function submit() {
  if (!canSave.value || props.saving || props.loading) return
  emit('save', normalizeStorageBackendForSubmit(form))
}

function healthBadge() {
  if (props.detail?.lastHealthOk === true) return { label: '正常', color: 'success' as const }
  if (props.detail?.lastHealthOk === false) return { label: '异常', color: 'error' as const }
  return { label: '未检查', color: 'neutral' as const }
}

function eventTypeLabel(type: string) {
  const labels: Record<string, string> = {
    config_upserted: '配置保存',
    config_deleted: '配置删除',
    health_check: '健康检查',
    secret_rotated: '密钥轮换'
  }
  return labels[type] || type
}

function eventStatusColor(status: string): 'success' | 'warning' | 'error' | 'neutral' {
  if (status === 'ok') return 'success'
  if (status === 'error') return 'error'
  if (status === 'warning') return 'warning'
  return 'neutral'
}

function briefDate(value?: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="isCOS ? '腾讯云 COS 存储后端' : '对象存储后端'"
    description="配置对象存储连接、访问凭据与运行状态。"
  >
    <template #body>
      <div class="space-y-4">
        <div v-if="editingName" class="rounded-lg border border-default bg-elevated/30 p-3">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium text-highlighted">{{ editingName }}</span>
                <UBadge :label="healthBadge().label" :color="healthBadge().color" variant="soft" size="sm" />
              </div>
              <div class="mt-1 text-xs text-muted">
                Secret v{{ detail?.secretVersion || 1 }}
                <span v-if="detail?.secretRotatedAt"> · 轮换 {{ briefDate(detail.secretRotatedAt) }}</span>
                <span v-if="detail?.lastHealthCheckedAt"> · 检查 {{ briefDate(detail.lastHealthCheckedAt) }}</span>
              </div>
              <p v-if="detail?.lastHealthError" class="mt-1 line-clamp-2 text-xs text-error">{{ detail.lastHealthError }}</p>
            </div>

            <div class="grid shrink-0 grid-cols-2 gap-2 sm:flex">
              <UButton
                icon="i-tabler-heartbeat"
                label="健康检查"
                color="neutral"
                variant="soft"
                size="xs"
                class="justify-center"
                :loading="checking"
                @click="emit('checkHealth')"
              />
              <UButton
                icon="i-tabler-key"
                label="轮换密钥"
                color="neutral"
                variant="soft"
                size="xs"
                class="justify-center"
                :disabled="loading"
                @click="emit('rotateSecret')"
              />
            </div>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="后端名称" required>
            <UInput v-model="form.name" :placeholder="isCOS ? 'tencent-cos' : 's3main'" class="w-full" :disabled="!!editingName || loading" />
          </UFormField>
          <UFormField label="后端类型" required>
            <USelectMenu v-model="form.type" :items="typeOptions" value-key="value" aria-label="后端类型" class="w-full" :disabled="!!editingName || loading" />
          </UFormField>
          <UFormField label="Region" required :hint="isCOS ? cosEndpoint : undefined">
            <UInput v-model="form.region" :placeholder="isCOS ? 'ap-shanghai' : 'us-east-1'" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField v-if="!isCOS" label="Endpoint" required>
            <UInput v-model="form.endpoint" placeholder="localhost:9000" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField v-if="isCOS" label="Bucket" required>
            <UInput v-model="cosBucket" placeholder="bucket-1250000000" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField v-if="!isCOS" label="Public Bucket" required>
            <UInput v-model="form.bucketPublic" placeholder="asset-public" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField v-if="!isCOS" label="Private Bucket" required>
            <UInput v-model="form.bucketPrivate" placeholder="asset-private" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField label="Access Key" required>
            <UInput v-model="form.accessKey" autocomplete="off" class="w-full" :disabled="loading" />
          </UFormField>
          <UFormField label="Secret Key" :required="!editingName">
            <UInput
              v-model="form.secretKey"
              type="password"
              autocomplete="new-password"
              class="w-full"
              :disabled="loading"
              :placeholder="editingName ? '留空沿用已有密钥' : ''"
            />
          </UFormField>

          <div class="flex flex-wrap items-center gap-x-5 gap-y-3 sm:col-span-2">
            <UCheckbox v-if="!isCOS" v-model="form.pathStyle" label="Path-style endpoint" :disabled="loading" />
            <UCheckbox v-if="!isCOS" v-model="form.useSsl" label="使用 HTTPS" :disabled="loading" />
            <span v-else class="text-xs text-muted">HTTPS · Virtual-hosted-style</span>
            <UCheckbox v-model="form.enabled" label="启用后端" :disabled="loading" />
          </div>

          <p class="text-xs text-muted sm:col-span-2">
            {{ isCOS ? 'Endpoint 将根据 Region 自动生成；同一个 Bucket 承担公开与私有对象角色。' : '保存时会先连接并注册后端；如果配置不可用，不会写入运行时。' }}
            编辑已有后端时 Secret Key 留空会沿用旧密钥。
          </p>
        </div>

        <div v-if="editingName" class="rounded-lg border border-default bg-default">
          <div class="flex items-center justify-between border-b border-default px-3 py-2">
            <h3 class="text-xs font-medium text-muted">最近事件</h3>
            <UButton
              icon="i-tabler-refresh"
              color="neutral"
              variant="ghost"
              square
              size="xs"
              aria-label="刷新最近事件"
              @click="emit('refreshEvents')"
            />
          </div>
          <ManageEmpty v-if="!events.length" icon="i-tabler-history" text="还没有后端事件" />
          <div v-else class="max-h-56 divide-y divide-default overflow-y-auto">
            <div v-for="event in events" :key="event.id" class="px-3 py-2.5">
              <div class="flex min-w-0 items-center gap-2">
                <span class="truncate text-sm font-medium text-highlighted">{{ eventTypeLabel(event.eventType) }}</span>
                <UBadge :label="event.status" :color="eventStatusColor(event.status)" variant="soft" size="sm" />
                <span class="ml-auto shrink-0 text-xs text-muted">{{ briefDate(event.createdAt) }}</span>
              </div>
              <div class="mt-0.5 truncate text-xs text-muted">{{ event.actor || 'system' }}<span v-if="event.message"> · {{ event.message }}</span></div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
        <UButton
          v-if="editingName"
          label="删除后端"
          icon="i-tabler-trash"
          color="error"
          variant="ghost"
          class="justify-center"
          :disabled="saving"
          @click="emit('delete')"
        />
        <span v-else />
        <div class="grid grid-cols-2 gap-2 sm:flex">
          <UButton label="取消" color="neutral" variant="ghost" class="justify-center" :disabled="saving" @click="() => { open = false }" />
          <UButton label="保存" icon="i-tabler-device-floppy" class="justify-center" :loading="saving" :disabled="!canSave || loading" @click="submit" />
        </div>
      </div>
    </template>
  </UModal>
</template>
