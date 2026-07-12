<script setup lang="ts">
withDefaults(defineProps<{
  backendName?: string
  rotating?: boolean
}>(), {
  backendName: '',
  rotating: false
})

const emit = defineEmits<{ confirm: [secret: string] }>()
const open = defineModel<boolean>('open', { required: true })
const secret = ref('')

watch(open, (isOpen) => {
  if (isOpen) secret.value = ''
})
</script>

<template>
  <UModal v-model:open="open" title="轮换存储后端密钥" description="新密钥保存成功后会记录一次安全事件。">
    <template #body>
      <div class="space-y-3">
        <p class="text-sm text-muted">
          为 <span class="font-medium text-default">{{ backendName }}</span> 写入新的 Secret Key。
        </p>
        <UFormField label="新的 Secret Key" required>
          <UInput v-model="secret" type="password" autocomplete="new-password" autofocus class="w-full" />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton color="neutral" variant="ghost" label="取消" class="justify-center" :disabled="rotating" @click="() => { open = false }" />
        <UButton color="primary" label="确认轮换" icon="i-tabler-key" class="justify-center" :loading="rotating" :disabled="!secret" @click="emit('confirm', secret)" />
      </div>
    </template>
  </UModal>
</template>
