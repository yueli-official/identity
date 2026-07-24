interface AdminStepUpState {
  open: boolean
  transactionId: string
  action: string
  resource: string
  expiresAt: string
  loading: boolean
  error: string
}

let resolveProof: ((proof: string) => void) | undefined
let rejectProof: ((error: Error) => void) | undefined

export function useAdminStepUp() {
  const { call } = useApi()
  const identityAudience = useRuntimeConfig().public.identityAudience
  const state = useState<AdminStepUpState>('admin-step-up', () => ({
    open: false,
    transactionId: '',
    action: '',
    resource: '',
    expiresAt: '',
    loading: false,
    error: '',
  }))

  async function authorize(action: string, resource: string): Promise<string> {
    if (resolveProof) cancel()
    const result = await call<{
      satisfied: boolean
      proof?: string
      transactionId?: string
      expiresAt?: string
      methods?: string[]
    }>('/api/v1/auth/step-up/begin', {
      method: 'POST',
      body: {
        audience: identityAudience,
        action,
        resource,
        requirement: {
          freshWithinSeconds: 300,
          minimumLevel: 'aal2',
          minimumProfile: 'urn:yueli:assurance:multi-factor',
          minimumFactorCount: 2,
        },
      },
    })
    if (result.satisfied && result.proof) return result.proof
    if (!result.transactionId || !result.methods?.includes('totp')) {
      throw new Error('当前账户没有可以满足此操作的双重验证方式。')
    }
    state.value = {
      open: true, transactionId: result.transactionId,
      action, resource, expiresAt: result.expiresAt ?? '',
      loading: false, error: '',
    }
    return new Promise<string>((resolve, reject) => {
      resolveProof = resolve
      rejectProof = reject
    })
  }

  async function finish(code: string) {
    state.value.loading = true
    state.value.error = ''
    try {
      const result = await call<{ proof: string }>('/api/v1/auth/step-up/totp', {
        method: 'POST',
        body: { transactionId: state.value.transactionId, code },
      })
      state.value.open = false
      resolveProof?.(result.proof)
      resolveProof = undefined
      rejectProof = undefined
    } catch (error) {
      state.value.error = mfaErrorMessage(error)
    } finally {
      state.value.loading = false
    }
  }

  function cancel() {
    state.value.open = false
    rejectProof?.(new Error('已取消额外身份验证。'))
    resolveProof = undefined
    rejectProof = undefined
  }

  async function run<T>(
    action: string,
    resource: string,
    operation: (proof: string) => Promise<T>,
  ): Promise<T> {
    const proof = await authorize(action, resource)
    return operation(proof)
  }

  return { state, authorize, finish, cancel, run }
}

export const adminStepUpResource = {
  role: (identityId: string, role: string) => `identity:${identityId}:role:${role.trim()}`,
  status: (identityId: string, status: string) => `identity:${identityId}:status:${status.trim()}`,
  identity: (identityId: string) => `identity:${identityId}`,
  create: (email: string, roles: string[]) =>
    `identity:new:${email.trim().toLowerCase()}:roles:${[...roles].sort().join(',')}`,
}
