<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'
import type { PasskeySupport } from '~/composables/usePasskeys'
import { safeReturnTo } from '~/utils/returnTo'

definePageMeta({ layout: 'auth', middleware: 'guest' })

const route = useRoute()
const { call } = useApi()
const { refresh } = useSession()
const error = ref('')
const loading = ref(false)
const passkeyLoading = ref(false)
const passkeySupport = ref<PasskeySupport>('unsupported')
const passkeyAvailable = computed(() => passkeySupport.value === 'supported')
const passkeys = usePasskeys()
const { data: externalProviderData } = await useAsyncData(
  'external-login-providers',
  () => call<{ entries: ExternalLoginProvider[] }>('/api/v1/auth/oauth/providers'),
  { default: () => ({ entries: [] }) },
)
const externalProviders = computed(() => externalProviderData.value.entries)
const mfaTransaction = ref(String(route.query.mfa_transaction ?? ''))
const mfaExpiresAt = ref('')
const mfaCode = ref('')
const mfaLoading = ref(false)
const useRecoveryCode = ref(false)
const mfaExpiry = useExpiryCountdown(mfaExpiresAt)
const recoveryCode = computed(() => parseRecoveryCode(mfaCode.value))
const recoveryCodeInputError = computed(() => {
  if (!useRecoveryCode.value || !mfaCode.value || recoveryCode.value) return undefined
  return '请输入一枚恢复代码，不要粘贴整组代码。'
})

watch(mfaExpiry.expired, (expired) => {
  if (!expired || !mfaTransaction.value) return
  mfaTransaction.value = ''
  mfaExpiresAt.value = ''
  mfaCode.value = ''
  useRecoveryCode.value = false
  error.value = '双重验证已过期，请重新输入邮箱和密码。'
})

onMounted(() => {
  passkeySupport.value = passkeys.support()
})

// Surface an error handed back via the query string (e.g. an OAuth redirect that
// failed) so the "Sign in with Google" button never silently appears dead.
const oauthError = computed(() => {
  return oauthRedirectErrorMessage(route.query.error, 'login', route.query.provider)
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
    const result = await call<{
      mfaRequired: boolean
      mfaTransaction?: string
      mfaExpiresAt?: string
      mfaMethods?: string[]
    }>('/api/v1/auth/login', { method: 'POST', body: e.data })
    if (result.mfaRequired && result.mfaTransaction) {
      mfaTransaction.value = result.mfaTransaction
      mfaExpiresAt.value = result.mfaExpiresAt ?? ''
      state.password = ''
      return
    }
    await refresh()
    // external: full-page nav so a return_to like /oauth2/authorize hits the
    // dev proxy → identity, instead of Nuxt's client router (which 404s — there
    // is no /oauth2/authorize page in this app).
    await navigateTo(safeReturnTo(route.query.return_to as string), { external: true })
  } catch (err: any) {
    error.value = identityErrorMessage(err, {
      context: 'login',
      fallback: '暂时无法登录。',
    })
  } finally {
    loading.value = false
  }
}

async function onMFASubmit() {
  if (!useRecoveryCode.value && !/^\d{6}$/.test(mfaCode.value)) return
  if (useRecoveryCode.value && !recoveryCode.value) return
  error.value = ''
  mfaLoading.value = true
  try {
    await call(useRecoveryCode.value ? '/api/v1/auth/mfa/recovery' : '/api/v1/auth/mfa/totp', {
      method: 'POST',
      body: {
        transactionId: mfaTransaction.value,
        code: useRecoveryCode.value ? recoveryCode.value : mfaCode.value,
      },
    })
    if (useRecoveryCode.value) {
      await navigateTo('/recovery')
      return
    }
    await refresh()
    await navigateTo(safeReturnTo(route.query.return_to as string), { external: true })
  } catch (err) {
    error.value = identityErrorMessage(err, {
      context: 'mfa',
      fallback: '暂时无法完成双重验证。',
    })
  } finally {
    mfaLoading.value = false
  }
}

function cancelMFA() {
  mfaTransaction.value = ''
  mfaExpiresAt.value = ''
  mfaCode.value = ''
  useRecoveryCode.value = false
  error.value = ''
}

function toggleRecoveryCode() {
  useRecoveryCode.value = !useRecoveryCode.value
  mfaCode.value = ''
  error.value = ''
}

async function onPasskeyLogin() {
  error.value = ''
  passkeyLoading.value = true
  try {
    await passkeys.authenticate()
    await refresh()
    await navigateTo(safeReturnTo(route.query.return_to as string), { external: true })
  } catch (err) {
    error.value = passkeyErrorMessage(err)
  } finally {
    passkeyLoading.value = false
  }
}

function cancelPasskeyLogin() {
  passkeys.cancelCeremony()
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

        <form v-if="mfaTransaction" class="space-y-4" @submit.prevent="onMFASubmit">
          <UAlert
            color="info"
            variant="soft"
            icon="i-tabler-shield-lock"
            :title="useRecoveryCode ? '使用恢复代码' : '完成双重验证'"
            :description="useRecoveryCode
              ? '恢复代码只能使用一次，登录后仅可重建身份验证器。'
              : '输入身份验证器应用中显示的 6 位动态验证码。'"
          />
          <p
            v-if="mfaExpiresAt"
            class="text-center text-xs text-muted"
            role="status"
            aria-live="polite"
          >
            验证将在 {{ mfaExpiry.label.value }} 后过期
          </p>
          <UFormField
            :label="useRecoveryCode ? '恢复代码' : '动态验证码'"
            :error="recoveryCodeInputError"
          >
            <UInput
              v-model="mfaCode"
              :inputmode="useRecoveryCode ? 'text' : 'numeric'"
              :autocomplete="useRecoveryCode ? 'off' : 'one-time-code'"
              :maxlength="useRecoveryCode ? 128 : 6"
              :pattern="useRecoveryCode ? undefined : '[0-9]{6}'"
              :placeholder="useRecoveryCode ? 'XXXX-XXXX-XXXX-XXXX' : undefined"
              :autocapitalize="useRecoveryCode ? 'characters' : undefined"
              :spellcheck="useRecoveryCode ? false : undefined"
              autofocus
              class="w-full"
            />
          </UFormField>
          <UAlert v-if="error" color="error" variant="soft" :title="error" />
          <UButton
            type="submit"
            label="验证并登录"
            block
            size="lg"
            :disabled="useRecoveryCode ? !recoveryCode : !/^\d{6}$/.test(mfaCode)"
            :loading="mfaLoading"
          />
          <UButton
            color="neutral"
            variant="link"
            :label="useRecoveryCode ? '改用动态验证码' : '无法使用身份验证器？使用恢复代码'"
            block
            :disabled="mfaLoading"
            @click="toggleRecoveryCode"
          />
          <UButton
            color="neutral"
            variant="ghost"
            label="返回其他登录方式"
            block
            :disabled="mfaLoading"
            @click="cancelMFA"
          />
        </form>

        <UForm v-else :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
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

        <USeparator v-if="!mfaTransaction" label="或" class="my-4" />
        <div v-if="!mfaTransaction" class="space-y-2">
          <UButton
            v-if="passkeyLoading"
            block
            color="neutral"
            variant="outline"
            icon="i-tabler-x"
            label="取消通行密钥登录"
            @click="cancelPasskeyLogin"
          />
          <UButton
            v-else
            block
            color="neutral"
            variant="outline"
            icon="i-tabler-key"
            label="使用通行密钥登录"
            :disabled="!passkeyAvailable || loading"
            @click="onPasskeyLogin"
          />
          <UButton
            v-for="provider in externalProviders"
            :key="provider.key"
            block
            color="neutral"
            variant="outline"
            :icon="externalLoginProviderMeta(provider.key).icon"
            :label="`使用 ${provider.label} 登录`"
            :to="`/api/v1/auth/oauth/${provider.key}/start?return_to=${encodeURIComponent(returnTo)}`"
            external
          />
        </div>
        <p v-if="!mfaTransaction && passkeySupport === 'insecure-context'" class="mt-3 text-center text-xs text-muted">
          当前地址不是安全连接；通行密钥登录需要 HTTPS 或 localhost
        </p>
        <p v-else-if="!mfaTransaction && !passkeyAvailable" class="mt-3 text-center text-xs text-muted">
          当前浏览器缺少通行密钥能力
        </p>
      </UCard>

      <p v-if="!mfaTransaction" class="text-center text-sm text-muted">
        还没有账户?<ULink :to="`/register?return_to=${encodeURIComponent(returnTo)}`" class="text-primary font-medium">注册</ULink>
      </p>
    </div>
  </div>
</template>
