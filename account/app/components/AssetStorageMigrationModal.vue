<script setup lang="ts">
import type { AssetStorageMigrationForm, AssetStorageMigrationResult } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetStorageMigrationForm
  preview?: AssetStorageMigrationResult | null
  backendOptions: SelectOption[]
  running?: boolean
}>(), {
  preview: null,
  running: false
})

const emit = defineEmits<{
  preview: [value: AssetStorageMigrationForm]
  confirm: [value: AssetStorageMigrationForm]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetStorageMigrationForm>({ ...props.initialValue })
const previewMatches = computed(() => (
  form.sourceBackend === props.initialValue.sourceBackend
  && form.targetBackend === props.initialValue.targetBackend
  && form.limit === props.initialValue.limit
))
const canPreview = computed(() => Boolean(
  form.sourceBackend
  && form.targetBackend
  && form.sourceBackend !== form.targetBackend
  && form.limit >= 1
  && form.limit <= 200
))
const canConfirm = computed(() => canPreview.value && previewMatches.value && Boolean(props.preview?.items?.length))

watch(open, (isOpen) => {
  if (isOpen) Object.assign(form, props.initialValue)
})
watch(() => props.initialValue, value => Object.assign(form, value), { deep: true })
</script>

<template>
  <UModal v-model:open="open" title="迁移存储后端" description="预检并迁移原文件与当前派生图，旧后端对象不会立即删除。">
    <template #body>
      <div class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-3">
          <UFormField label="源后端" required>
            <USelectMenu v-model="form.sourceBackend" :items="backendOptions" value-key="value" class="w-full" :search-input="{ placeholder: '搜索源后端…' }" />
          </UFormField>
          <UFormField label="目标后端" required>
            <USelectMenu v-model="form.targetBackend" :items="backendOptions" value-key="value" class="w-full" :search-input="{ placeholder: '搜索目标后端…' }" />
          </UFormField>
          <UFormField label="单次上限" required>
            <UInput v-model.number="form.limit" type="number" min="1" max="200" class="w-full" />
          </UFormField>
        </div>
        <p class="text-sm text-muted">
          成功后会更新素材所属后端；旧对象可在确认稳定后通过孤儿对象扫描清理。
        </p>
        <UAlert v-if="preview && !previewMatches" color="warning" variant="soft" icon="i-tabler-alert-triangle" title="参数已变化" description="请重新预检后再执行迁移。" />
        <AssetMaintenanceCandidateList
          :items="previewMatches ? preview?.items : []"
          :candidates="previewMatches ? preview?.candidates : 0"
          :limit="form.limit"
          kind="migration"
          empty-icon="i-tabler-database-off"
          :empty-text="previewMatches ? '还没有预检结果' : '等待重新预检'"
        />
      </div>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="running" @click="() => { open = false }" />
        <UButton color="neutral" variant="soft" label="预检" icon="i-tabler-search" class="justify-center" :loading="running" :disabled="!canPreview" @click="emit('preview', { ...form })" />
        <UButton label="确认迁移" icon="i-tabler-transfer" class="justify-center" :loading="running" :disabled="!canConfirm" @click="emit('confirm', { ...form })" />
      </div>
    </template>
  </UModal>
</template>
