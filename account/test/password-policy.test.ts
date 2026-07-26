import { describe, expect, it } from 'vitest'
import {
  newPasswordSchema,
  passwordLength,
  passwordMeetsLengthPolicy,
  PASSWORD_HINT,
} from '../app/utils/password'

describe('password policy client guard', () => {
  it('counts NFC-normalized Unicode code points', () => {
    expect(passwordLength('Cafe\u0301')).toBe(4)
    expect(passwordLength('🔐'.repeat(15))).toBe(15)
  })

  it('matches the server 8–128 character boundary', () => {
    expect(passwordMeetsLengthPolicy('界'.repeat(7))).toBe(false)
    expect(passwordMeetsLengthPolicy('界'.repeat(8))).toBe(true)
    expect(passwordMeetsLengthPolicy('界'.repeat(128))).toBe(true)
    expect(passwordMeetsLengthPolicy('界'.repeat(129))).toBe(false)
  })

  it('provides the same boundary through form schemas', () => {
    expect(newPasswordSchema().safeParse('short').success).toBe(false)
    expect(newPasswordSchema().safeParse('correct horse battery').success).toBe(true)
  })

  it('summarizes the active boundary and blocklist', () => {
    expect(PASSWORD_HINT).toContain('8–128')
    expect(PASSWORD_HINT).toContain('常见或泄露密码不可用')
  })
})
