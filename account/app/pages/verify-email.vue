<script setup lang="ts">
definePageMeta({ layout: 'auth' })

const route = useRoute()
const { call } = useApi()
const status = ref<'pending' | 'success' | 'failure'>('pending')

onMounted(async () => {
  const token = (route.query.token as string) || ''
  if (!token) {
    status.value = 'failure'
    return
  }
  try {
    await call('/api/v1/auth/email/verify', { method: 'POST', body: { token } })
    status.value = 'success'
  } catch {
    status.value = 'failure'
  }
})
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm text-center">
      <template #header>
        <h1 class="font-display text-xl font-semibold text-highlighted">邮箱验证</h1>
      </template>

      <template v-if="status === 'pending'">
        <div class="flex flex-col items-center gap-2 py-4 text-muted">
          <UIcon name="i-lucide-loader-circle" class="size-6 animate-spin" />
          <p class="text-sm">正在验证…</p>
        </div>
      </template>

      <template v-else-if="status === 'success'">
        <UAlert color="success" variant="soft" title="邮箱已验证" description="你的邮箱地址已成功验证。" />
        <UButton to="/" label="前往账户中心" block class="mt-4" />
      </template>

      <template v-else>
        <UAlert color="error" variant="soft" title="链接无效或已过期" description="请重新登录后再次发送验证邮件。" />
        <UButton to="/login" label="返回登录" block class="mt-4" />
      </template>
    </UCard>
  </div>
</template>
