<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'auth' })

const { call } = useApi()
const loading = ref(false)
const submitted = ref(false)

const schema = z.object({
  email: z.email('邮箱格式不正确')
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ email: '' })

async function onSubmit(e: FormSubmitEvent<Schema>) {
  loading.value = true
  try {
    await call('/api/v1/auth/password/forgot', { method: 'POST', body: e.data })
  } catch {
    // Swallow errors: the confirmation is intentionally identical regardless of
    // whether the email exists, to avoid account enumeration.
  } finally {
    loading.value = false
    submitted.value = true
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <UCard class="w-full max-w-sm">
      <template #header>
        <h1 class="font-display text-xl font-semibold text-highlighted">找回密码</h1>
        <p class="mt-1 text-sm text-muted">输入邮箱,我们将发送重置链接</p>
      </template>

      <template v-if="submitted">
        <UAlert
          color="success"
          variant="soft"
          title="请检查你的邮箱"
          description="若该邮箱已注册,我们已向其发送重置链接。"
        />
      </template>
      <UForm v-else :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
        <UFormField name="email" label="邮箱">
          <UInput v-model="state.email" type="email" autocomplete="email" placeholder="you@example.com" class="w-full" />
        </UFormField>
        <UButton type="submit" label="发送重置链接" block :loading="loading" />
      </UForm>

      <template #footer>
        <p class="text-center text-sm text-muted">
          想起来了?<ULink to="/login" class="text-primary font-medium">返回登录</ULink>
        </p>
      </template>
    </UCard>
  </div>
</template>
