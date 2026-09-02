<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import { safeReturnTo } from '~/utils/returnTo'
import { externalLoginProviderMeta } from '~/utils/external-login'
import { PASSWORD_HINT } from '~/utils/password'

definePageMeta({ layout: 'auth' })

const route = useRoute()
const { call } = useApi()
const { refresh } = useSession()
const error = ref('')
const loading = ref(false)
const { data: externalProviderData } = await useAsyncData(
  'external-login-providers',
  () => call<{ entries: ExternalLoginProvider[] }>('/api/v1/auth/oauth/providers'),
  { default: () => ({ entries: [] }) },
)
const registrationProviders = computed(() =>
  externalProviderData.value.entries.filter(
    provider => provider.registrationPolicy === 'verified_email',
  ),
)

const oauthError = computed(() => {
  return oauthRedirectErrorMessage(route.query.error, 'register', route.query.provider)
})

const returnTo = computed(() => String(route.query.return_to ?? '/'))

const schema = z.object({
  displayName: z.string().min(1, '请输入昵称'),
  email: z.email('邮箱格式不正确'),
  password: newPasswordSchema()
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
    error.value = identityErrorMessage(err, {
      context: 'register',
      fallback: '暂时无法创建账户。',
    })
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
          <UFormField name="password" label="密码" :hint="PASSWORD_HINT">
            <UInput v-model="state.password" type="password" autocomplete="new-password" class="w-full" />
          </UFormField>
          <UAlert v-if="error" color="error" variant="soft" :title="error" />
          <UButton type="submit" label="注册" block size="lg" :loading="loading" />
        </UForm>

        <USeparator v-if="registrationProviders.length" label="或" class="my-4" />
        <UButton
          v-for="provider in registrationProviders"
          :key="provider.key"
          block
          color="neutral"
          variant="outline"
          :icon="externalLoginProviderMeta(provider.key).icon"
          :label="`使用 ${provider.label} 注册`"
          :to="`/api/v1/auth/oauth/${provider.key}/start?return_to=${encodeURIComponent(returnTo)}`"
          external
        />
      </UCard>

      <p class="text-center text-sm text-muted">
        已有账户?<ULink :to="`/login?return_to=${encodeURIComponent(returnTo)}`" class="text-primary font-medium">登录</ULink>
      </p>
    </div>
  </div>
</template>
