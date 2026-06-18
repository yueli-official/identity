<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

const { me } = useSession()
const { call } = useApi()
const toast = useToast()
const resending = ref(false)

const initial = computed(() =>
  (me.value?.displayName || me.value?.email || '?').charAt(0).toUpperCase()
)

async function onResendVerification() {
  resending.value = true
  try {
    await call('/api/v1/auth/email/verify-request', { method: 'POST' })
    toast.add({
      title: '验证邮件已发送',
      description: '请检查邮箱完成验证。(demo 环境无真实邮件:链接打印在后端日志)',
      color: 'success',
      icon: 'i-lucide-mail-check'
    })
  } catch (err: any) {
    toast.add({ title: '发送失败', description: err?.data?.message || '请稍后重试。', color: 'error' })
  } finally {
    resending.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Identity header -->
    <div class="flex items-center gap-4">
      <UAvatar
        :text="initial"
        :src="me?.avatarUrl || undefined"
        size="3xl"
        class="ring-2 ring-primary/20"
      />
      <div class="min-w-0">
        <h1 class="font-display truncate text-2xl font-semibold text-highlighted">
          {{ me?.displayName || '我的账户' }}
        </h1>
        <p class="truncate text-sm text-muted">{{ me?.email }}</p>
      </div>
    </div>

    <UAlert
      v-if="me && !me.emailVerified"
      color="warning"
      variant="soft"
      icon="i-lucide-mail-warning"
      title="邮箱尚未验证"
      description="验证邮箱以保护账户安全。"
    >
      <template #actions>
        <UButton
          color="warning"
          size="sm"
          label="重新发送验证邮件"
          :loading="resending"
          @click="onResendVerification"
        />
      </template>
    </UAlert>

    <!-- Account info -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-id-card" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">账户信息</h2>
        </div>
      </template>
      <dl class="text-sm">
        <div class="flex items-center justify-between gap-4 border-b border-default py-2.5">
          <dt class="text-muted">昵称</dt>
          <dd class="text-highlighted">{{ me?.displayName || '—' }}</dd>
        </div>
        <div class="flex items-center justify-between gap-4 border-b border-default py-2.5">
          <dt class="text-muted">用户名</dt>
          <dd class="text-highlighted">{{ me?.username || '—' }}</dd>
        </div>
        <div class="flex items-center justify-between gap-4 border-b border-default py-2.5">
          <dt class="shrink-0 text-muted">邮箱</dt>
          <dd class="flex min-w-0 items-center gap-2">
            <span class="truncate text-highlighted">{{ me?.email }}</span>
            <UBadge :color="me?.emailVerified ? 'success' : 'warning'" variant="soft" size="sm">
              {{ me?.emailVerified ? '已验证' : '未验证' }}
            </UBadge>
          </dd>
        </div>
        <div class="flex items-center justify-between gap-4 py-2.5">
          <dt class="shrink-0 text-muted">账户 ID</dt>
          <dd class="truncate font-mono text-xs text-dimmed">{{ me?.id }}</dd>
        </div>
      </dl>
    </UCard>

    <!-- Security -->
    <UCard class="shadow-soft">
      <template #header>
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-shield" class="size-5 text-muted" />
          <h2 class="font-medium text-highlighted">安全</h2>
        </div>
      </template>
      <div class="flex items-center justify-between gap-4">
        <div class="min-w-0">
          <p class="text-sm text-highlighted">密码</p>
          <p class="text-xs text-muted">通过邮件链接重置你的登录密码</p>
        </div>
        <UButton to="/forgot" color="neutral" variant="outline" size="sm" label="重置密码" />
      </div>
    </UCard>
  </div>
</template>
