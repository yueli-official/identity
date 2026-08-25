import { getApiFailure } from '@yueli/http-runtime'
import { identityErrorMessage } from '~/utils/api-errors'

interface AccountReauthenticationState {
  open: boolean
  loading: boolean
  error: string
}

interface PendingReauthentication {
  retry: () => Promise<unknown>
  resolve: (value: unknown) => void
  reject: (error: Error) => void
}

let pending: PendingReauthentication | undefined

export class AccountReauthenticationCancelledError extends Error {
  readonly code = 'account.reauthentication_cancelled'
}

export function isAccountReauthenticationCancelled(error: unknown) {
  return error instanceof AccountReauthenticationCancelledError
}

export function useAccountReauthentication() {
  const { call } = useApi()
  const state = useState<AccountReauthenticationState>('account-reauthentication', () => ({
    open: false,
    loading: false,
    error: '',
  }))

  async function run<T>(operation: () => Promise<T>): Promise<T> {
    try {
      return await operation()
    } catch (error) {
      if (getApiFailure(error)?.code !== 'identity.step_up_required') throw error
      if (pending) cancel()
      state.value = { open: true, loading: false, error: '' }
      return new Promise<T>((resolve, reject) => {
        pending = {
          retry: operation,
          resolve: value => resolve(value as T),
          reject,
        }
      })
    }
  }

  async function verify(password: string) {
    if (!pending || !password) return
    state.value.loading = true
    state.value.error = ''
    try {
      await call('/api/v1/auth/reauthenticate', {
        method: 'POST',
        body: { password },
      })
    } catch (error) {
      state.value.loading = false
      state.value.error = identityErrorMessage(error, {
        context: 'password-change',
        fallback: '暂时无法重新验证身份。',
      })
      return
    }

    const active = pending
    pending = undefined
    state.value = { open: false, loading: false, error: '' }
    try {
      active.resolve(await active.retry())
    } catch (error) {
      active.reject(error instanceof Error ? error : new Error('安全操作失败'))
    }
  }

  function cancel() {
    const active = pending
    pending = undefined
    state.value = { open: false, loading: false, error: '' }
    active?.reject(new AccountReauthenticationCancelledError('已取消重新验证身份'))
  }

  return { state, run, verify, cancel }
}
