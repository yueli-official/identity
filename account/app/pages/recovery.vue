<script setup lang="ts">
definePageMeta({ layout: 'auth' })

const { call } = useApi()
const completed = ref(false)
const leaving = ref(false)
const { data, pending, error } = await useAsyncData(
  'restricted-recovery-session',
  () => call<{ identityId: string, expiresAt: string }>('/api/v1/account/recovery'),
)
const recoveryExpiry = useExpiryCountdown(() => data.value?.expiresAt ?? '')
const recoveryUnavailable = computed(() => !!error.value || !data.value || recoveryExpiry.expired.value)

async function returnToLogin() {
  leaving.value = true
  try {
    await call('/api/v1/auth/logout', { method: 'POST' })
  } finally {
    await navigateTo('/login')
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <div class="w-full max-w-2xl space-y-6">
      <div class="text-center">
        <span class="mx-auto grid size-12 place-items-center rounded-2xl bg-warning/10 text-warning">
          <UIcon name="i-tabler-lifebuoy" class="size-6" />
        </span>
        <h1 class="font-display mt-3 text-xl font-semibold text-highlighted">恢复账户安全</h1>
        <p class="mt-1 text-sm text-muted">此会话不能访问资料、角色、令牌或其他账户功能。</p>
      </div>

      <UCard v-if="pending">
        <USkeleton class="h-32 w-full rounded-lg" />
      </UCard>
      <UAlert
        v-else-if="recoveryUnavailable"
        color="error"
        variant="soft"
        icon="i-tabler-clock-x"
        title="恢复会话无效或已过期"
        description="请重新使用密码和一枚未使用的恢复代码。"
      >
        <template #actions>
          <UButton to="/login" label="返回登录" />
        </template>
      </UAlert>
      <UAlert
        v-else-if="completed"
        color="success"
        variant="soft"
        icon="i-tabler-shield-check"
        title="身份验证器已重建"
        description="原有身份验证器和恢复代码已经失效。请退出恢复会话，然后使用新验证器正常登录。"
      >
        <template #actions>
          <UButton label="退出并重新登录" :loading="leaving" @click="returnToLogin" />
        </template>
      </UAlert>
      <template v-else>
        <UAlert
          color="warning"
          variant="soft"
          icon="i-tabler-lock-access"
          title="受限恢复会话"
          :description="`会话将在 ${recoveryExpiry.label.value} 后过期，只能用于重建身份验证器。`"
          aria-live="polite"
        />
        <TOTPManager recovery @recovered="completed = true" />
      </template>
    </div>
  </div>
</template>
