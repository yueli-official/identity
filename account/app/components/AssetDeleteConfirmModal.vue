<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  description: string
  subject?: string
  deleting?: boolean
}>(), {
  subject: '',
  deleting: false
})

const emit = defineEmits<{ confirm: [] }>()
const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <UModal v-model:open="open" :title="title" :description="description">
    <template #body>
      <p v-if="subject" class="break-all text-sm font-medium text-default">{{ subject }}</p>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="deleting" @click="() => { open = false }" />
        <UButton color="error" label="确认删除" icon="i-tabler-trash" class="justify-center" :loading="deleting" @click="emit('confirm')" />
      </div>
    </template>
  </UModal>
</template>
