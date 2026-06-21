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

const oauthError = computed(() => {
  const e = route.query.error
  if (!e) return ''
  if (e === 'oauth_unavailable') return 'Google 登录在该环境未配置,请用邮箱注册。'
  return '第三方登录失败,请重试或改用邮箱注册。'
})

const returnTo = computed(() => String(route.query.return_to ?? '/'))

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
    // external: full-page nav so an /oauth2/authorize return_to goes through the
    // dev proxy → identity, not Nuxt's client router (which has no such page).
    await navigateTo(safeReturnTo(route.query.return_to as string), { external: true })
  } catch (err: any) {
    error.value = err?.data?.message || '注册失败,请重试'
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
          <UIcon name="i-tabler-user-plus" class="size-6" />
        </span>
        <h1 class="font-display mt-3 text-xl font-semibold text-highlighted">创建账户</h1>
        <p class="mt-1 text-sm text-muted">加入账户中心</p>
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
          <UButton type="submit" label="注册" block size="lg" :loading="loading" />
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
        已有账户?<ULink :to="`/login?return_to=${encodeURIComponent(returnTo)}`" class="text-primary font-medium">登录</ULink>
      </p>
    </div>
  </div>
</template>
