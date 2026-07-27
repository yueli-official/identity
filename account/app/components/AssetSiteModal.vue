<script setup lang="ts">
import type { AssetSite } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetSite
  storageBackendOptions: SelectOption[]
  saving?: boolean
}>(), {
  saving: false
})

const emit = defineEmits<{
  save: [value: AssetSite]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetSite>({ ...props.initialValue })
const canSave = computed(() => Boolean(
  form.siteKey.trim()
  && form.name.trim()
  && form.defaultStorageBackend.trim()
))

watch(open, (isOpen) => {
  if (isOpen) Object.assign(form, props.initialValue)
})

function submit() {
  if (!canSave.value || props.saving) return
  emit('save', { ...form })
}
</script>

<template>
  <UModal v-model:open="open" title="站点配置" description="设置站点标识、默认存储后端与上传状态。">
    <template #body>
      <div class="grid gap-4 sm:grid-cols-2">
        <UFormField label="Site Key" required>
          <UInput v-model="form.siteKey" placeholder="blog" class="w-full" />
        </UFormField>

        <UFormField label="站点名称" required>
          <UInput v-model="form.name" placeholder="Blog" class="w-full" />
        </UFormField>

        <UFormField label="默认存储后端" required>
          <USelectMenu
            v-model="form.defaultStorageBackend"
            :items="storageBackendOptions"
            value-key="value"
            class="w-full"
            :search-input="{ placeholder: '搜索后端…' }"
          />
        </UFormField>

        <UFormField label="状态">
          <div class="flex min-h-8 items-center">
            <USwitch v-model="form.enabled" label="允许上传" />
          </div>
        </UFormField>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton
          label="取消"
          color="neutral"
          variant="ghost"
          class="justify-center"
          :disabled="saving"
          @click="() => { open = false }"
        />
        <UButton
          label="保存"
          icon="i-tabler-device-floppy"
          class="justify-center"
          :loading="saving"
          :disabled="!canSave"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>
