<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import { safeReturnTo } from '~/utils/returnTo'

const route = useRoute()
const { call } = useApi()
const { refresh } = useSession()
const error = ref('')
const loading = ref(false)

const schema = z.object({
  displayName: z.string().min(1, '请输入昵称'),
  email: z.email('邮箱格式不正确'),
  password: z.string().min(8, '密码至少 8 位').max(128, '密码最多 128 位')
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ displayName: '', email: '', password: '' })

async function onSubmit(e: FormSubmitEvent<Schema>) {
  error.value = ''
  loading.value = true
  try {
    await call('/api/v1/auth/register', { method: 'POST', body: e.data })
    await call('/api/v1/auth/login', { method: 'POST', body: { email: e.data.email, password: e.data.password } })
    await refresh()
    await navigateTo(safeReturnTo(route.query.return_to as string))
  } catch (err: any) {
    error.value = err?.data?.message || '注册失败,请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm">
      <template #header>
        <h1 class="font-display text-xl font-semibold text-highlighted">创建账户</h1>
        <p class="mt-1 text-sm text-muted">加入我们</p>
      </template>
      <UForm :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormField name="displayName" label="昵称">
          <UInput v-model="state.displayName" autocomplete="nickname" class="w-full" />
        </UFormField>
        <UFormField name="email" label="邮箱">
          <UInput v-model="state.email" type="email" autocomplete="email" placeholder="you@example.com" class="w-full" />
        </UFormField>
        <UFormField name="password" label="密码" hint="至少 8 位">
          <UInput v-model="state.password" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <UAlert v-if="error" color="error" variant="soft" :title="error" />
        <UButton type="submit" label="注册" block :loading="loading" />
      </UForm>
      <template #footer>
        <p class="text-center text-sm text-muted">
          已有账户?<ULink to="/login" class="text-primary font-medium">登录</ULink>
        </p>
      </template>
    </UCard>
  </div>
</template>
