<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
const { me, logout } = useSession()
const { call } = useApi()
const toast = useToast()
const resending = ref(false)

async function onLogout() {
  await logout()
  await navigateTo('/login')
}

async function onResendVerification() {
  resending.value = true
  try {
    await call('/api/v1/auth/email/verify-request', { method: 'POST' })
    toast.add({ title: '验证邮件已发送', description: '请检查你的邮箱并点击链接完成验证。', color: 'success' })
  } catch (err: any) {
    toast.add({ title: '发送失败', description: err?.data?.message || '请稍后重试。', color: 'error' })
  } finally {
    resending.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl p-6 space-y-6">
    <h1 class="font-display text-2xl font-semibold text-highlighted">账户中心</h1>
    <UAlert
      v-if="me && !me.emailVerified"
      color="warning"
      variant="soft"
      title="邮箱尚未验证"
      description="请验证你的邮箱以保护账户安全。"
    >
      <template #actions>
        <UButton
          color="warning"
          variant="solid"
          size="sm"
          label="重新发送验证邮件"
          :loading="resending"
          @click="onResendVerification"
        />
      </template>
    </UAlert>
    <UCard v-if="me">
      <div class="space-y-2">
        <p class="text-sm"><span class="text-muted">ID:</span> {{ me.id }}</p>
        <p class="text-sm">
          <span class="text-muted">邮箱:</span> {{ me.email }}
          <UBadge :color="me.emailVerified ? 'success' : 'warning'" variant="soft" class="ml-2">
            {{ me.emailVerified ? '已验证' : '未验证' }}
          </UBadge>
        </p>
      </div>
      <template #footer>
        <UButton color="neutral" variant="outline" label="退出登录" @click="onLogout" />
      </template>
    </UCard>
  </div>
</template>
