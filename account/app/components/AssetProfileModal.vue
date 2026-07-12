<script setup lang="ts">
import type { AssetProfile, AssetSite } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetProfile
  sites: AssetSite[]
  siteOptions: SelectOption[]
  storageBackendOptions: SelectOption[]
  accessLevelOptions: SelectOption[]
  saving?: boolean
}>(), {
  saving: false
})

const emit = defineEmits<{
  save: [value: AssetProfile]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetProfile>({ ...props.initialValue })

const storageOptions = computed(() => {
  const site = props.sites.find(item => item.siteKey === form.siteKey)
  const inheritedBackend = site?.defaultStorageBackend || 'local'
  return [
    { label: `继承站点默认 (${inheritedBackend})`, value: '' },
    ...props.storageBackendOptions
  ]
})

const canSave = computed(() => Boolean(
  form.siteKey.trim()
  && form.profileKey.trim()
  && form.allowedExt.trim()
  && form.maxSizeBytes > 0
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
  <UModal v-model:open="open" title="Profile 配置" description="配置资源用途、存储策略与上传约束。">
    <template #body>
      <div class="grid gap-4 sm:grid-cols-2">
        <UFormField label="站点" required>
          <USelectMenu
            v-model="form.siteKey"
            :items="siteOptions"
            value-key="value"
            class="w-full"
          />
        </UFormField>

        <UFormField label="Profile Key" required>
          <UInput v-model="form.profileKey" placeholder="blog-cover" class="w-full" />
        </UFormField>

        <UFormField label="用途" class="sm:col-span-2">
          <UInput v-model="form.purpose" placeholder="文章封面 / 正文图片 / 付费资源" class="w-full" />
        </UFormField>

        <UFormField label="存储后端" class="sm:col-span-2">
          <USelectMenu
            v-model="form.storageBackend"
            :items="storageOptions"
            value-key="value"
            class="w-full"
            :search-input="{ placeholder: '搜索后端…' }"
          />
        </UFormField>

        <UFormField label="允许后缀" required>
          <UInput v-model="form.allowedExt" placeholder="jpg,jpeg,png,webp" class="w-full" />
        </UFormField>

        <UFormField label="大小上限（bytes）" required>
          <UInput v-model.number="form.maxSizeBytes" type="number" min="1" class="w-full" />
        </UFormField>

        <UFormField
          label="访问级别"
          help="公开资源返回稳定公开地址；私有资源由业务授权后生成签名链接。"
        >
          <USelect v-model="form.defaultVisibility" :items="accessLevelOptions" value-key="value" class="w-full" />
        </UFormField>

        <div class="flex items-end pb-1">
          <UCheckbox v-model="form.keepOriginal" label="保留原图/原文件" />
        </div>
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
