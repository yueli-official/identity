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
  email: z.email('邮箱格式不正确'),
  password: z.string().min(1, '请输入密码')
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ email: '', password: '' })

async function onSubmit(e: FormSubmitEvent<Schema>) {
  error.value = ''
  loading.value = true
  try {
    await call('/api/v1/auth/login', { method: 'POST', body: e.data })
    await refresh()
    await navigateTo(safeReturnTo(route.query.return_to as string))
  } catch (err: any) {
    error.value = err?.data?.message || '登录失败,请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm">
      <template #header>
        <h1 class="font-display text-xl font-semibold text-highlighted">欢迎回来</h1>
        <p class="mt-1 text-sm text-muted">登录你的账户</p>
      </template>

      <UForm :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormField name="email" label="邮箱">
          <UInput v-model="state.email" type="email" autocomplete="email" placeholder="you@example.com" class="w-full" />
        </UFormField>
        <UFormField name="password" label="密码">
          <template #hint>
            <ULink to="/forgot" class="text-primary text-sm">忘记密码?</ULink>
          </template>
          <UInput v-model="state.password" type="password" autocomplete="current-password" class="w-full" />
        </UFormField>
        <UAlert v-if="error" color="error" variant="soft" :title="error" />
        <UButton type="submit" label="登录" block :loading="loading" />
      </UForm>

      <template #footer>
        <p class="text-center text-sm text-muted">
          还没有账户?<ULink :to="`/register?return_to=${encodeURIComponent(String(route.query.return_to ?? '/'))}`" class="text-primary font-medium">注册</ULink>
        </p>
      </template>
    </UCard>
  </div>
</template>
