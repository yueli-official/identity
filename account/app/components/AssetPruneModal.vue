<script setup lang="ts">
import type { AssetPruneForm, AssetPruneResult } from '~/types/asset-admin'

const props = withDefaults(defineProps<{
  initialValue: AssetPruneForm
  preview?: AssetPruneResult | null
  running?: boolean
}>(), {
  preview: null,
  running: false
})

const emit = defineEmits<{
  preview: [value: AssetPruneForm]
  confirm: [value: AssetPruneForm]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetPruneForm>({ ...props.initialValue })
const previewMatches = computed(() => form.olderThanDays === props.initialValue.olderThanDays && form.limit === props.initialValue.limit)
const canPreview = computed(() => form.olderThanDays >= 1 && form.limit >= 1 && form.limit <= 200)
const canConfirm = computed(() => canPreview.value && previewMatches.value && Boolean(props.preview?.items?.length))

watch(open, (isOpen) => {
  if (isOpen) Object.assign(form, props.initialValue)
})

watch(() => props.initialValue, value => Object.assign(form, value), { deep: true })
</script>

<template>
  <UModal v-model:open="open" title="清理无引用素材" description="先预检候选素材，再执行不可恢复的删除任务。">
    <template #body>
      <div class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-2">
          <UFormField label="保留天数" required>
            <UInput v-model.number="form.olderThanDays" type="number" min="1" class="w-full" />
          </UFormField>
          <UFormField label="单次上限" required>
            <UInput v-model.number="form.limit" type="number" min="1" max="200" class="w-full" />
          </UFormField>
        </div>
        <p class="text-sm text-muted">
          只处理超过 {{ form.olderThanDays }} 天且没有业务引用的素材；执行时服务端会再次检查引用。
        </p>
        <UAlert v-if="preview && !previewMatches" color="warning" variant="soft" icon="i-tabler-alert-triangle" title="参数已变化" description="请重新预检后再执行清理。" />
        <AssetMaintenanceCandidateList
          :items="previewMatches ? preview?.items : []"
          :candidates="previewMatches ? preview?.candidates : 0"
          :limit="form.limit"
          kind="prune"
          empty-icon="i-tabler-unlink"
          :empty-text="previewMatches ? '没有可清理的无引用素材' : '等待重新预检'"
        />
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
