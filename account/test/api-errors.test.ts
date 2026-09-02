import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  apiErrorMessage,
  IDENTITY_ERROR_MESSAGES,
  identityErrorMessage,
  oauthRedirectErrorMessage,
  pageErrorMessage,
} from '../app/utils/api-errors'

function remoteFailure(
  code: string,
  params: Readonly<Record<string, string | number | boolean>> = {},
  status = 400,
) {
  return {
    data: {
      failure: {
        kind: 'remote',
        status,
        code,
        params,
        violations: [],
        traceId: 'trace-contract-test',
        reauth: 'not-attempted',
      },
    },
  }
}

describe('Account API error presentation', () => {
  it('covers every registered Identity error code', () => {
    const catalog = JSON.parse(readFileSync(
      new URL('../../contracts/errors/catalog.json', import.meta.url),
      'utf8',
    )) as { errors: Array<{ code: string }> }
    const identityCodes = catalog.errors
      .map(entry => entry.code)
      .filter(code => code.startsWith('identity.'))

    expect(identityCodes.filter(code => !(code in IDENTITY_ERROR_MESSAGES))).toEqual([])
  })

  it.each([
    ['too_short', '至少需要 8 个字符'],
    ['too_long', '不能超过 128 个字符'],
    ['blocklisted', '过于常见或已出现在泄露名单'],
    ['context_specific', '不能与邮箱、邮箱前缀或昵称相同'],
  ])('maps current Problem params for weak-password reason %s', (reason, expected) => {
    expect(identityErrorMessage(
      remoteFailure('identity.weak_password', { reason }),
      { context: 'password-change' },
    )).toContain(expected)
  })

  it('uses operation context for the same stable code', () => {
    const failure = remoteFailure('identity.invalid_credentials', {}, 401)
    expect(identityErrorMessage(failure, { context: 'login' }))
      .toBe('邮箱或密码不正确。')
    expect(identityErrorMessage(failure, { context: 'password-change' }))
      .toBe('当前密码不正确。')
    expect(identityErrorMessage(
      remoteFailure('identity.mfa_transaction_invalid', {}, 401),
      { context: 'admin' },
    )).toBe('额外身份验证已过期，请重新发起操作。')
  })

  it.each([
    ['display_name_required', '昵称不能为空'],
    ['file_required', '请选择要上传的图片'],
    ['image_too_large', '图片过大'],
    ['unsupported_image', '不是支持的图片格式'],
    ['upload_unreadable', '无法读取所选图片'],
  ])('maps stable invalid-profile reason %s', (reason, expected) => {
    expect(identityErrorMessage(
      remoteFailure('identity.invalid_profile', { reason }),
      { context: 'profile' },
    )).toContain(expected)
  })

  it('shows the server-provided retry time for throttled requests', () => {
    const message = identityErrorMessage(remoteFailure(
      'identity.reset_throttled',
      { retryAt: '2026-07-26T12:34:56Z' },
      429,
    ))
    expect(message).toContain('可在')
    expect(message).toContain('2026')
    expect(message).not.toContain('请可在')
  })

  it('uses stable machine-flow reasons and indexes', () => {
    expect(identityErrorMessage(remoteFailure(
      'identity.pat_scope_invalid',
      { reason: 'invalid_scope', index: 2 },
    ))).toContain('第 3 个')
    expect(identityErrorMessage(remoteFailure(
      'identity.publisher_attestation_invalid',
      { reason: 'attestation_invalid' },
    ))).toContain('签名')
  })

  it('maps OAuth redirect codes instead of collapsing them', () => {
    expect(oauthRedirectErrorMessage('oauth_state', 'login')).toContain('已失效')
    expect(oauthRedirectErrorMessage('oauth_denied', 'register')).toContain('已取消')
    expect(oauthRedirectErrorMessage('oauth_bind', 'bind')).toContain('可能已绑定')
  })

  it('does not expose a raw framework message on the error page', () => {
    expect(pageErrorMessage({ statusCode: 404, message: 'internal route detail' }))
      .toBe('没有找到这个页面。')
    expect(pageErrorMessage({ statusCode: 500, message: 'database password leaked' }))
      .not.toContain('database')
  })

  it('gives removed or expired passkeys an actionable conservative message', () => {
    expect(identityErrorMessage(
      remoteFailure('identity.passkey_ceremony_invalid'),
      { context: 'passkey' },
    )).toContain('请改用其他登录方式')
  })

  it.each([
    ['network', 'foundation.request.network', '无法连接服务器'],
    ['timeout', 'foundation.request.timeout', '请求超时'],
  ] as const)('distinguishes local %s failures', (kind, code, expected) => {
    expect(apiErrorMessage({
      data: {
        failure: {
          kind,
          code,
          reauth: 'not-attempted',
        },
      },
    })).toContain(expected)
  })

  it('keeps a trace reference for unknown remote failures', () => {
    expect(apiErrorMessage(remoteFailure('identity.future_failure', {}, 500)))
      .toContain('trace-contract-test')
  })
})
