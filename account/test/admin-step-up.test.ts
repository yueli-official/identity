import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  adminStepUpFailureMessage,
  type AdminStepUpInterruptionReason,
} from '../app/utils/admin-step-up-errors'

interface StepUpState {
  open: boolean
  transactionId: string
  action: string
  resource: string
  expiresAt: string
  loading: boolean
  error: string
}

const call = vi.fn()
let state: { value: StepUpState } | undefined

async function createStepUp() {
  const { useAdminStepUp } = await import('../app/composables/useAdminStepUp')
  return useAdminStepUp()
}

describe('administrator step-up flow', () => {
  beforeEach(() => {
    vi.resetModules()
    call.mockReset()
    state = undefined
    vi.stubGlobal('useApi', () => ({ call }))
    vi.stubGlobal('useRuntimeConfig', () => ({
      public: { identityAudience: 'identity-api' },
    }))
    vi.stubGlobal('useState', (_key: string, factory: () => StepUpState) => {
      state ??= { value: factory() }
      return state
    })
  })

  it.each([
    ['cancelled', undefined],
    ['expired', '额外身份验证已过期，请重新发起操作。'],
  ] as const)(
    '%s interruption closes the challenge and never runs the protected operation',
    async (
      reason: AdminStepUpInterruptionReason,
      expectedMessage: string | undefined,
    ) => {
      call.mockResolvedValue({
        satisfied: false,
        transactionId: 'transaction-1',
        expiresAt: '2026-07-26T12:05:00Z',
        methods: ['totp'],
      })
      const stepUp = await createStepUp()
      const operation = vi.fn()
      const pending = stepUp.run(
        'identity.admin.status.update',
        'identity:user-1:status:disabled',
        operation,
      )

      await vi.waitFor(() => expect(stepUp.state.value.open).toBe(true))
      stepUp.cancel(reason)

      const error = await pending.catch(cause => cause)
      expect(error).toMatchObject({
        name: 'AdminStepUpInterruptedError',
        reason,
      })
      expect(adminStepUpFailureMessage(error, '暂时无法更新账户状态。'))
        .toBe(expectedMessage)
      expect(stepUp.state.value.open).toBe(false)
      expect(operation).not.toHaveBeenCalled()
    },
  )
})
