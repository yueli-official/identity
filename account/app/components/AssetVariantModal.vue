<script setup lang="ts">
import type { AssetVariant } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  initialValue: AssetVariant
  modeOptions: SelectOption[]
  saving?: boolean
}>(), {
  saving: false
})

const emit = defineEmits<{
  save: [value: AssetVariant]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetVariant>({ ...props.initialValue })
const canSave = computed(() => Boolean(
  form.siteKey.trim()
  && form.profileKey.trim()
  && form.variantKey.trim()
  && form.width > 0
  && form.height > 0
  && form.quality > 0
  && form.quality <= 100
  && form.version > 0
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
  <UModal v-model:open="open" title="Variant 规则" description="定义该 Profile 的派生尺寸、处理模式与版本。">
    <template #body>
      <div class="grid gap-4 sm:grid-cols-2">
        <UFormField label="站点">
          <UInput v-model="form.siteKey" disabled class="w-full" />
        </UFormField>

        <UFormField label="Profile">
          <UInput v-model="form.profileKey" disabled class="w-full" />
        </UFormField>

        <UFormField label="Variant Key" required>
          <UInput v-model="form.variantKey" placeholder="card / og / content" class="w-full" />
        </UFormField>

        <UFormField label="模式">
          <USelect v-model="form.mode" :items="modeOptions" value-key="value" class="w-full" />
        </UFormField>

        <UFormField label="宽度" required>
          <UInput v-model.number="form.width" type="number" min="1" class="w-full" />
        </UFormField>

        <UFormField label="高度" required>
          <UInput v-model.number="form.height" type="number" min="1" class="w-full" />
        </UFormField>

        <UFormField label="质量" required help="请输入 1–100。">
          <UInput v-model.number="form.quality" type="number" min="1" max="100" class="w-full" />
        </UFormField>

        <UFormField label="版本" required>
          <UInput v-model.number="form.version" type="number" min="1" class="w-full" />
        </UFormField>

        <div class="flex items-end pb-1 sm:col-span-2">
          <UCheckbox v-model="form.enabled" label="启用" />
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
