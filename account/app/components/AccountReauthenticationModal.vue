<script setup lang="ts">
const props = defineProps<{ hasPassword: boolean }>()
const reauthentication = useAccountReauthentication()
const state = reauthentication.state
const password = ref('')

watch(
  () => state.value.open,
  (open) => {
    if (open) password.value = ''
  },
)

function cancel() {
  password.value = ''
  reauthentication.cancel()
}

async function verify() {
  await reauthentication.verify(password.value)
  if (!state.value.open) password.value = ''
}
</script>

<template>
  <UModal
    :open="state.open"
    :dismissible="false"
    title="重新验证身份"
    description="继续修改账户安全设置。"
    :ui="{ content: 'w-full max-w-md bg-default' }"
    @update:open="value => !value && cancel()"
  >
    <template #body>
      <form v-if="props.hasPassword" class="space-y-4" @submit.prevent="verify">
        <p class="text-sm leading-6 text-muted">
          为保护账户，请输入当前密码。验证完成后会自动继续刚才的操作。
        </p>
        <UFormField label="当前密码">
          <UInput
            v-model="password"
            type="password"
            autocomplete="current-password"
            autofocus
            class="w-full"
          />
        </UFormField>
        <UAlert
          v-if="state.error"
          color="error"
          variant="soft"
          icon="i-tabler-alert-circle"
          :title="state.error"
        />
        <div class="flex justify-end gap-2 pt-1">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            label="取消"
            :disabled="state.loading"
            @click="cancel"
          />
          <UButton
            type="submit"
            label="验证并继续"
            :loading="state.loading"
            :disabled="!password"
          />
        </div>
      </form>

      <div v-else class="space-y-4">
        <UAlert
          color="warning"
          variant="soft"
          icon="i-tabler-login-2"
          title="需要重新登录"
          description="当前账户没有密码。请使用原登录方式重新登录后，再立即完成这项安全设置。"
        />
        <div class="flex justify-end">
          <UButton color="neutral" variant="outline" label="关闭" @click="cancel" />
        </div>
      </div>
    </template>
  </UModal>
</template>
