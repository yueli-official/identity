<script setup lang="ts">
withDefaults(defineProps<{
  backendName?: string
  deleting?: boolean
}>(), {
  backendName: '',
  deleting: false
})

const emit = defineEmits<{ confirm: [] }>()
const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <UModal v-model:open="open" title="删除存储后端？" description="仅未被站点、Profile 或素材使用的后端可以删除。">
    <template #body>
      <p class="text-sm text-muted">
        将删除 <span class="font-medium text-default">{{ backendName }}</span>。此操作不会删除远端对象存储中的文件。
      </p>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="deleting" @click="() => { open = false }" />
        <UButton color="error" label="确认删除" class="justify-center" :loading="deleting" @click="emit('confirm')" />
      </div>
    </template>
  </UModal>
</template>
