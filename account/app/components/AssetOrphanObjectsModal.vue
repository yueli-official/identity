<script setup lang="ts">
import { ManageEmpty } from '@platform/manage/components'
import type { AssetOrphanObjectForm, AssetOrphanObjectResult } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetOrphanObjectForm
  preview?: AssetOrphanObjectResult | null
  backendOptions: SelectOption[]
  running?: boolean
}>(), {
  preview: null,
  running: false
})

const emit = defineEmits<{
  preview: [value: AssetOrphanObjectForm]
  confirm: [value: AssetOrphanObjectForm]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetOrphanObjectForm>({ ...props.initialValue })
const previewMatches = computed(() => (
  form.backend === props.initialValue.backend
  && form.olderThanDays === props.initialValue.olderThanDays
  && form.limit === props.initialValue.limit
))
const canPreview = computed(() => Boolean(form.backend && form.olderThanDays >= 1 && form.limit >= 1 && form.limit <= 500))
const canConfirm = computed(() => canPreview.value && previewMatches.value && Boolean(props.preview?.orphans))

watch(open, (isOpen) => {
  if (isOpen) Object.assign(form, props.initialValue)
})
watch(() => props.initialValue, value => Object.assign(form, value), { deep: true })

function formatBytes(value: number) {
  if (!value) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function briefDate(value?: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}
</script>

<template>
  <UModal v-model:open="open" title="扫描孤儿对象" description="对比对象存储与数据库记录，预检后再清理多余对象。">
    <template #body>
      <div class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-3">
          <UFormField label="存储后端" required>
            <USelectMenu v-model="form.backend" :items="backendOptions" value-key="value" class="w-full" :search-input="{ placeholder: '搜索后端…' }" />
          </UFormField>
          <UFormField label="保留天数" required>
            <UInput v-model.number="form.olderThanDays" type="number" min="1" class="w-full" />
          </UFormField>
          <UFormField label="返回上限" required>
            <UInput v-model.number="form.limit" type="number" min="1" max="500" class="w-full" />
          </UFormField>
        </div>
        <p class="text-sm text-muted">扫描 public/private 对象并与原文件及当前派生图 key 对比，只处理超过 {{ form.olderThanDays }} 天的多余对象。</p>
        <UAlert v-if="preview && !previewMatches" color="warning" variant="soft" icon="i-tabler-alert-triangle" title="参数已变化" description="请重新预检后再执行清理。" />

        <div class="space-y-3">
          <div v-for="backend in previewMatches ? preview?.items ?? [] : []" :key="backend.backend" class="overflow-hidden rounded-lg border border-default bg-default">
            <div class="flex flex-col gap-2 border-b border-default px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="line-clamp-1 text-sm font-medium text-highlighted">{{ backend.backend }}</div>
              <div class="flex flex-wrap items-center gap-2">
                <UBadge :label="backend.skipped ? '不支持扫描' : `扫描 ${backend.scanned}`" :color="backend.skipped ? 'neutral' : 'primary'" variant="soft" />
                <UBadge :label="`期望 ${backend.expected}`" color="neutral" variant="soft" />
                <UBadge :label="`孤儿 ${backend.orphans}`" :color="backend.orphans ? 'warning' : 'success'" variant="soft" />
              </div>
            </div>
            <div v-if="backend.error" class="px-3 py-2 text-sm text-error">{{ backend.error }}</div>
            <ManageEmpty v-else-if="!backend.items?.length" icon="i-tabler-database-check" text="没有发现可清理对象" />
            <div v-else class="max-h-64 divide-y divide-default overflow-y-auto overflow-x-hidden">
              <div v-for="item in backend.items" :key="item.key" class="flex min-w-0 items-center gap-3 px-3 py-2.5">
                <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-elevated text-muted"><UIcon name="i-tabler-file-database" class="size-4" /></span>
                <div class="min-w-0 flex-1">
                  <div class="line-clamp-1 font-mono text-xs text-highlighted">{{ item.key }}</div>
                  <div class="mt-0.5 text-xs text-muted">{{ formatBytes(item.size) }} · {{ briefDate(item.modTime) }}</div>
                </div>
              </div>
            </div>
          </div>
          <ManageEmpty v-if="!previewMatches || !preview?.items?.length" icon="i-tabler-database-search" :text="previewMatches ? '还没有扫描结果' : '等待重新预检'" />
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="running" @click="() => { open = false }" />
        <UButton color="neutral" variant="soft" label="重新预检" icon="i-tabler-search" class="justify-center" :loading="running" :disabled="!canPreview" @click="emit('preview', { ...form })" />
        <UButton color="error" label="确认清理" icon="i-tabler-trash" class="justify-center" :loading="running" :disabled="!canConfirm" @click="emit('confirm', { ...form })" />
      </div>
    </template>
  </UModal>
</template>
