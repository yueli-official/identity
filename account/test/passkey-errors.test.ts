import { describe, expect, it } from 'vitest'
import { passkeyErrorMessage } from '../app/composables/usePasskeys'

describe('passkeyErrorMessage', () => {
  it('distinguishes an explicit cancellation from browser timeout or unavailable credentials', () => {
    expect(passkeyErrorMessage(new DOMException('aborted', 'AbortError'))).toBe('操作已取消。')
    expect(passkeyErrorMessage(new DOMException('not allowed', 'NotAllowedError')))
      .toContain('操作已取消、超时')
  })

  it('preserves an API error message', () => {
    expect(passkeyErrorMessage({ data: { message: '服务端拒绝了凭据' } }))
      .toBe('服务端拒绝了凭据')
  })
})
