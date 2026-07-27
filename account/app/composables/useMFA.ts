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

const recoveryCodePattern = /^[A-Z2-7]{16}$/
const recoveryCodeSeparators = /[\s\-\u2010-\u2015\u2212\uFE58\uFE63\uFF0D]/g

export function parseRecoveryCode(value: string): string | undefined {
  const canonical = value
    .normalize('NFKC')
    .toUpperCase()
    .replace(recoveryCodeSeparators, '')
  return recoveryCodePattern.test(canonical) ? canonical : undefined
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
