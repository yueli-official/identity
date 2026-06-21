<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import { safeReturnTo } from '~/utils/returnTo'

definePageMeta({ layout: 'auth' })

const route = useRoute()
const { call } = useApi()
const { refresh } = useSession()
const error = ref('')
const loading = ref(false)

// Surface an error handed back via the query string (e.g. an OAuth redirect that
// failed) so the "Sign in with Google" button never silently appears dead.
const oauthError = computed(() => {
  const e = route.query.error
  if (!e) return ''
  if (e === 'oauth_unavailable') return 'Google 登录在该环境未配置,请用邮箱登录。'
  return '第三方登录失败,请重试或改用邮箱登录。'
})

const returnTo = computed(() => String(route.query.return_to ?? '/'))

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
    // external: full-page nav so a return_to like /oauth2/authorize hits the
    // dev proxy → identity, instead of Nuxt's client router (which 404s — there
    // is no /oauth2/authorize page in this app).
    await navigateTo(safeReturnTo(route.query.return_to as string), { external: true })
  } catch (err: any) {
    error.value = err?.data?.message || '登录失败,请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center p-4">
    <div class="w-full max-w-sm space-y-6">
      <div class="text-center">
        <span class="mx-auto grid size-12 place-items-center rounded-2xl bg-primary/10 text-primary">
          <UIcon name="i-tabler-shield-check" class="size-6" />
        </span>
        <h1 class="font-display mt-3 text-xl font-semibold text-highlighted">欢迎回来</h1>
        <p class="mt-1 text-sm text-muted">登录账户中心</p>
      </div>

      <UCard class="shadow-soft">
        <UAlert
          v-if="oauthError"
          color="warning"
          variant="soft"
          icon="i-tabler-info-circle"
          :description="oauthError"
          class="mb-4"
        />

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
          <UButton type="submit" label="登录" block size="lg" :loading="loading" />
        </UForm>

        <USeparator label="或" class="my-4" />
        <UButton
          block
          color="neutral"
          variant="outline"
          icon="i-tabler-brand-google"
          label="使用 Google 登录"
          :to="`/api/v1/auth/oauth/google/start?return_to=${encodeURIComponent(returnTo)}`"
          external
        />
      </UCard>

      <p class="text-center text-sm text-muted">
        还没有账户?<ULink :to="`/register?return_to=${encodeURIComponent(returnTo)}`" class="text-primary font-medium">注册</ULink>
      </p>
    </div>
  </div>
</template>
