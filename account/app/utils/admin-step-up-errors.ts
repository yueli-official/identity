import { identityErrorMessage } from './api-errors'

export type AdminStepUpInterruptionReason = 'cancelled' | 'expired'

export class AdminStepUpInterruptedError extends Error {
  readonly code = 'account.admin_step_up_interrupted'
  readonly reason: AdminStepUpInterruptionReason

  constructor(reason: AdminStepUpInterruptionReason) {
    super(
      reason === 'expired'
        ? '额外身份验证已过期，请重新发起操作。'
        : '已取消额外身份验证。',
    )
    this.name = 'AdminStepUpInterruptedError'
    this.reason = reason
  }
}

export function isAdminStepUpInterruptedError(
  error: unknown,
): error is AdminStepUpInterruptedError {
  if (!error || typeof error !== 'object') return false
  const candidate = error as { code?: unknown, reason?: unknown }
  return candidate.code === 'account.admin_step_up_interrupted'
    && (candidate.reason === 'cancelled' || candidate.reason === 'expired')
}

export function adminStepUpFailureMessage(
  error: unknown,
  fallback: string,
): string | undefined {
  if (isAdminStepUpInterruptedError(error)) {
    return error.reason === 'expired' ? error.message : undefined
  }
  return identityErrorMessage(error, { context: 'admin', fallback })
}
