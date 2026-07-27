<script setup lang="ts">
import type { AssetGrantForm, CreatedAssetGrant } from '~/types/asset-admin'

interface SelectOption {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  assetId?: string
  initialValue: AssetGrantForm
  createdGrant?: CreatedAssetGrant | null
  policyOptions: SelectOption[]
  creating?: boolean
}>(), {
  assetId: '',
  createdGrant: null,
  creating: false
})

const emit = defineEmits<{
  create: [value: AssetGrantForm]
}>()

const open = defineModel<boolean>('open', { required: true })
const form = reactive<AssetGrantForm>({ ...props.initialValue })
const copied = ref(false)
let copyResetTimer: ReturnType<typeof setTimeout> | undefined

const canCreate = computed(() => Boolean(
  props.assetId
  && form.variantKey.trim()
  && form.policy.trim()
  && form.expiresIn >= 60
  && form.maxUses >= 1
))

watch(open, (isOpen) => {
  if (!isOpen) return
  Object.assign(form, props.initialValue)
  copied.value = false
})

onBeforeUnmount(() => {
  if (copyResetTimer) clearTimeout(copyResetTimer)
})

function submit() {
  if (!canCreate.value || props.creating) return
  emit('create', { ...form })
}

async function copyLink() {
  if (!props.createdGrant?.url) return
  await navigator.clipboard.writeText(props.createdGrant.url)
  copied.value = true
  if (copyResetTimer) clearTimeout(copyResetTimer)
  copyResetTimer = setTimeout(() => { copied.value = false }, 2000)
}

function briefDate(value: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-'
}
</script>

<template>
  <UModal v-model:open="open" title="签发交付链接" description="为指定素材创建受策略、时效和使用次数约束的访问链接。">
    <template #body>
      <div class="space-y-4">
        <p class="truncate font-mono text-xs text-dimmed">{{ assetId }}</p>

        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Variant" required>
            <UInput v-model="form.variantKey" placeholder="original / card / cover" class="w-full" />
          </UFormField>

          <UFormField label="策略" required>
            <USelect v-model="form.policy" :items="policyOptions" value-key="value" class="w-full" />
          </UFormField>

          <UFormField label="有效期（秒）" required>
            <UInput v-model.number="form.expiresIn" type="number" min="60" class="w-full" />
          </UFormField>

          <UFormField label="最大使用次数" required>
            <UInput v-model.number="form.maxUses" type="number" min="1" class="w-full" />
          </UFormField>

          <UFormField label="Subject ID" class="sm:col-span-2">
            <UInput v-model="form.subjectId" placeholder="留空表示不绑定用户" class="w-full" />
          </UFormField>

          <UFormField label="原因" class="sm:col-span-2">
            <UInput v-model="form.reason" class="w-full" />
          </UFormField>
        </div>

        <p class="text-xs text-muted">有效期最多 24 小时，最大使用次数最多 100 次；服务端会自动收紧超出的值。</p>

        <div v-if="createdGrant" class="rounded-lg border border-default bg-default p-3">
          <div class="mb-2 flex items-center justify-between gap-2">
            <div class="text-sm font-medium text-highlighted">交付链接</div>
            <UButton
              :icon="copied ? 'i-tabler-check' : 'i-tabler-copy'"
              :label="copied ? '已复制' : '复制'"
              :color="copied ? 'success' : 'neutral'"
              variant="soft"
              size="xs"
              @click="copyLink"
            />
          </div>
          <p class="break-all font-mono text-xs text-muted">{{ createdGrant.url }}</p>
          <p class="mt-2 text-xs text-muted">过期 {{ briefDate(createdGrant.expiresAt) }}</p>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <UButton
          color="neutral"
          variant="ghost"
          label="关闭"
          class="justify-center"
          :disabled="creating"
          @click="() => { open = false }"
        />
        <UButton
          label="生成链接"
          icon="i-tabler-key"
          class="justify-center"
          :loading="creating"
          :disabled="!canCreate"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>
