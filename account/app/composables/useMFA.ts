export interface TOTPEntry {
  id: string
  label: string
  status: string
  createdAt: string
  verifiedAt?: string
  lastUsedAt?: string
}

export interface TOTPEnrollment {
  authenticatorId: string
  uri: string
  secret: string
  expiresAt: string
}

export function mfaErrorMessage(error: unknown): string {
  const candidate = error as any
  const code = candidate?.data?.code ?? candidate?.data?.data?.code
  if (code === 'identity.totp_code_invalid') return '验证码无效、已过期或已经使用，请等待新验证码后重试。'
  if (code === 'identity.totp_enrollment_invalid') return '设置过程已过期，请重新开始。'
  if (code === 'identity.step_up_required') return '这项安全操作需要重新验证身份。'
  if (code === 'identity.recovery_code_invalid') return '恢复代码无效或已经使用。'
  if (code === 'identity.mfa_transaction_invalid') return '登录验证已过期，请重新登录。'
  return candidate?.data?.message
    ?? candidate?.data?.data?.message
    ?? candidate?.statusMessage
    ?? '身份验证器操作失败，请重试。'
}

export function useMFA() {
  const { call } = useApi()

  function listTOTP() {
    return call<{ entries: TOTPEntry[] }>('/api/v1/account/mfa/totp')
  }

  function beginTOTP(label: string) {
    return call<TOTPEnrollment>('/api/v1/account/mfa/totp/enrollment/begin', {
      method: 'POST',
      body: { label },
    })
  }

  function finishTOTP(authenticatorId: string, code: string) {
    return call<{ authenticator: TOTPEntry, recoveryCodes: string[] }>(
      '/api/v1/account/mfa/totp/enrollment/finish',
      { method: 'POST', body: { authenticatorId, code } },
    )
  }

  function removeTOTP(id: string) {
    return call(`/api/v1/account/mfa/totp/${id}`, { method: 'DELETE' })
  }

  return { listTOTP, beginTOTP, finishTOTP, removeTOTP }
}
