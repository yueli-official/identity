<script setup lang="ts">
import type { AssetItem } from '~/types/asset-admin'

const props = withDefaults(defineProps<{
  asset?: AssetItem | null
  rejecting?: boolean
}>(), {
  asset: null,
  rejecting: false
})

const emit = defineEmits<{ confirm: [reason: string] }>()
const open = defineModel<boolean>('open', { required: true })
const reason = ref('')
const canConfirm = computed(() => reason.value.trim().length >= 3 && reason.value.trim().length <= 500)

watch(open, (value) => {
  if (value) reason.value = ''
})

function closeModal() {
  open.value = false
}

function confirm() {
  if (!canConfirm.value || props.rejecting) return
  emit('confirm', reason.value.trim())
}
</script>

<template>
  <UModal v-model:open="open" title="拒绝素材交付？" description="素材会保持不可交付，并保留隔离原文供后续重试或删除。">
    <template #body>
      <div class="space-y-4">
        <p v-if="asset" class="break-all text-sm font-medium text-default">{{ asset.filename || asset.id }}</p>
        <UFormField label="处置原因" help="原因会进入安全投影与审计记录。" required>
          <UTextarea
            v-model="reason"
            :rows="4"
            :maxlength="500"
            placeholder="例如：内容不符合本站发布政策"
            class="w-full"
            :disabled="rejecting"
            autofocus
          />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton label="取消" color="neutral" variant="ghost" class="justify-center" :disabled="rejecting" @click="closeModal" />
        <UButton
          label="确认拒绝"
          icon="i-tabler-user-shield"
          color="warning"
          class="justify-center"
          :loading="rejecting"
          :disabled="!canConfirm"
          @click="confirm"
        />
      </div>
    </template>
  </UModal>
</template>
