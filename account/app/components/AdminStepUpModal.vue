<script setup lang="ts">
const stepUp = useAdminStepUp()
const code = ref('')
const expiry = useExpiryCountdown(() => stepUp.state.value.expiresAt)

watch(() => stepUp.state.value.open, (open) => {
  if (open) code.value = ''
})

watch(expiry.expired, (expired) => {
  if (!expired || !stepUp.state.value.open) return
  code.value = ''
  stepUp.cancel('expired')
})

function cancel() {
  code.value = ''
  stepUp.cancel()
}

async function submit() {
  if (!/^\d{6}$/.test(code.value)) return
  await stepUp.finish(code.value)
}
</script>

<template>
  <UModal
    :open="stepUp.state.value.open"
    :dismissible="false"
    title="确认高风险操作"
    description="此操作需要近期的双重身份验证。"
  >
    <template #body>
      <form class="space-y-4" @submit.prevent="submit">
        <UAlert
          color="warning"
          variant="soft"
          icon="i-tabler-shield-lock"
          title="输入动态验证码"
          description="验证仅授权当前操作和目标，不能用于其他管理操作。"
        />
        <p
          v-if="stepUp.state.value.expiresAt"
          class="text-center text-xs text-muted"
          role="status"
          aria-live="polite"
        >
          验证将在 {{ expiry.label.value }} 后过期
        </p>
        <UFormField label="6 位动态验证码">
          <UInput
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            pattern="[0-9]{6}"
            autofocus
            class="w-full"
          />
        </UFormField>
        <UAlert v-if="stepUp.state.value.error" color="error" variant="soft" :title="stepUp.state.value.error" />
        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="stepUp.state.value.loading"
            @click="cancel"
          />
          <UButton
            type="submit"
            label="验证"
            :disabled="!/^\d{6}$/.test(code)"
            :loading="stepUp.state.value.loading"
          />
        </div>
      </form>
    </template>
  </UModal>
</template>
