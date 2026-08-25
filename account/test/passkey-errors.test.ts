import { describe, expect, it } from 'vitest'
import { passkeyErrorMessage, resolvePasskeySupport } from '../app/composables/usePasskeys'

describe('passkeyErrorMessage', () => {
  it('distinguishes an explicit cancellation from browser timeout or unavailable credentials', () => {
    expect(passkeyErrorMessage(new DOMException('aborted', 'AbortError'))).toBe('操作已取消。')
    expect(passkeyErrorMessage(new DOMException('not allowed', 'NotAllowedError')))
      .toContain('操作已取消、超时')
  })

  it('translates the current passkey Problem contract', () => {
    expect(passkeyErrorMessage({
      data: {
        failure: {
          kind: 'remote',
          status: 400,
          code: 'identity.passkey_ceremony_invalid',
          params: {},
          violations: [],
          traceId: 'passkey-test',
          reauth: 'not-attempted',
        },
      },
    })).toContain('请改用其他登录方式')
  })
})

describe('resolvePasskeySupport', () => {
  it('distinguishes an insecure origin from a browser capability gap', () => {
    expect(resolvePasskeySupport({
      secureContext: false,
      credentialAPI: false,
      jsonHelpers: false,
    })).toBe('insecure-context')
    expect(resolvePasskeySupport({
      secureContext: true,
      credentialAPI: false,
      jsonHelpers: false,
    })).toBe('unsupported')
    expect(resolvePasskeySupport({
      secureContext: true,
      credentialAPI: true,
      jsonHelpers: true,
    })).toBe('supported')
  })
})
