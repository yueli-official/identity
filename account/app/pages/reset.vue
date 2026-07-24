<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'auth' })

const route = useRoute()
const { call } = useApi()
const token = computed(() => (route.query.token as string) || '')
const error = ref('')
const loading = ref(false)
const done = ref(false)

const schema = z.object({
  password: newPasswordSchema(),
  confirm: z.string().min(1, '请再次输入密码')
}).refine(data => data.password === data.confirm, {
  message: '两次输入的密码不一致',
  path: ['confirm']
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ password: '', confirm: '' })

async function onSubmit(e: FormSubmitEvent<Schema>) {
  error.value = ''
  loading.value = true
  try {
    await call('/api/v1/auth/password/reset', { method: 'POST', body: { token: token.value, password: e.data.password } })
    done.value = true
    await navigateTo('/login')
  } catch (err: any) {
    error.value = err?.data?.message || '链接无效或已过期,请重新申请。'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm">
      <template #header>
        <h1 class="font-display text-xl font-semibold text-highlighted">重置密码</h1>
        <p class="mt-1 text-sm text-muted">设置一个新密码</p>
      </template>

      <template v-if="!token">
        <UAlert color="error" variant="soft" title="链接无效" description="缺少重置令牌,请重新申请重置链接。" />
        <UButton to="/forgot" variant="link" label="重新申请" class="mt-4" />
      </template>

      <template v-else-if="done">
        <UAlert color="success" variant="soft" title="密码已重置" description="正在跳转到登录页…" />
      </template>

      <UForm v-else :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormField name="password" label="新密码" :hint="PASSWORD_HINT">
          <UInput v-model="state.password" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <UFormField name="confirm" label="确认密码">
          <UInput v-model="state.confirm" type="password" autocomplete="new-password" class="w-full" />
        </UFormField>
        <UAlert v-if="error" color="error" variant="soft" :title="error">
          <template #description>
            <ULink to="/forgot" class="text-primary font-medium">重新申请重置链接</ULink>
          </template>
        </UAlert>
        <UButton type="submit" label="重置密码" block :loading="loading" />
      </UForm>

      <template #footer>
        <p class="text-center text-sm text-muted">
          <ULink to="/login" class="text-primary font-medium">返回登录</ULink>
        </p>
      </template>
    </UCard>
  </div>
</template>
