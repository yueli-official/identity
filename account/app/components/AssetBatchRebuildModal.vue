<script setup lang="ts">
import type { AssetBatchRebuildResult, AssetProfile } from '~/types/asset-admin'

const props = withDefaults(defineProps<{
  profile?: AssetProfile | null
  preview?: AssetBatchRebuildResult | null
  previewLimit: number
  running?: boolean
}>(), {
  profile: null,
  preview: null,
  running: false
})

const emit = defineEmits<{
  preview: [limit: number]
  confirm: [limit: number]
}>()

const open = defineModel<boolean>('open', { required: true })
const limit = ref(props.previewLimit)
const previewMatches = computed(() => limit.value === props.previewLimit)
const canPreview = computed(() => limit.value >= 1 && limit.value <= 200)
const canConfirm = computed(() => canPreview.value && previewMatches.value && Boolean(props.preview?.items?.length))

watch(open, (isOpen) => {
  if (isOpen) limit.value = props.previewLimit
})
watch(() => props.previewLimit, value => { limit.value = value })
</script>

<template>
  <UModal v-model:open="open" title="批量重建派生图" description="先预检图片素材，再排队重建当前 Variant。">
    <template #body>
      <div class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-[1fr_140px]">
          <UFormField label="Profile">
            <UInput :model-value="profile ? `${profile.siteKey} / ${profile.profileKey}` : ''" disabled class="w-full" />
          </UFormField>
          <UFormField label="单次上限" required>
            <UInput v-model.number="limit" type="number" min="1" max="200" class="w-full" />
          </UFormField>
        </div>
        <p class="text-sm text-muted">会重新生成该 Profile 下图片素材的当前 Variant；建议先小批量验证规则。</p>
        <UAlert v-if="preview && !previewMatches" color="warning" variant="soft" icon="i-tabler-alert-triangle" title="上限已变化" description="请重新预检后再确认重建。" />
        <AssetMaintenanceCandidateList
          :items="previewMatches ? preview?.items : []"
          :candidates="previewMatches ? preview?.candidates : 0"
          :limit="limit"
          kind="rebuild"
          empty-icon="i-tabler-photo-off"
          :empty-text="previewMatches ? '没有可重建的图片素材' : '等待重新预检'"
        />
      </div>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="running" @click="() => { open = false }" />
        <UButton color="neutral" variant="soft" label="重新预检" icon="i-tabler-search" class="justify-center" :loading="running" :disabled="!canPreview" @click="emit('preview', limit)" />
        <UButton label="确认重建" icon="i-tabler-refresh-dot" class="justify-center" :loading="running" :disabled="!canConfirm" @click="emit('confirm', limit)" />
      </div>
    </template>
  </UModal>
</template>
