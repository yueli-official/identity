<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
const { me, logout } = useSession()
async function onLogout() {
  await logout()
  await navigateTo('/login')
}
</script>

<template>
  <div class="mx-auto max-w-2xl p-6 space-y-6">
    <h1 class="font-display text-2xl font-semibold text-highlighted">账户中心</h1>
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
