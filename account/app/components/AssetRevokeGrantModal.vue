<script setup lang="ts">
const props = withDefaults(defineProps<{
  assetId?: string
  revoking?: boolean
}>(), {
  assetId: '',
  revoking: false
})

const emit = defineEmits<{
  confirm: []
}>()

const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <UModal v-model:open="open" title="撤销授权？" description="撤销后，已发出的一次性或门禁链接会立即失效。">
    <template #body>
      <p class="text-sm text-muted">此操作不会删除素材，但无法恢复当前授权。</p>
      <p class="mt-2 truncate font-mono text-xs text-dimmed">{{ props.assetId }}</p>
    </template>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton
          color="neutral"
          variant="ghost"
          label="取消"
          class="justify-center"
          :disabled="revoking"
          @click="() => { open = false }"
        />
        <UButton
          color="error"
          label="确认撤销"
          class="justify-center"
          :loading="revoking"
          @click="emit('confirm')"
        />
      </div>
    </template>
  </UModal>
</template>
