import { describe, expect, it } from 'vitest'
import { expirySeconds, formatExpiryCountdown } from '../app/composables/useExpiryCountdown'

describe('ceremony expiry countdown', () => {
  it('rounds remaining partial seconds up and clamps expired values', () => {
    const now = Date.parse('2026-07-24T00:00:00.000Z')
    expect(expirySeconds('2026-07-24T00:01:00.001Z', now)).toBe(61)
    expect(expirySeconds('2026-07-23T23:59:59Z', now)).toBe(0)
    expect(expirySeconds('invalid', now)).toBe(0)
  })

  it('formats minutes and seconds for assistive text', () => {
    expect(formatExpiryCountdown(125)).toBe('02:05')
    expect(formatExpiryCountdown(0)).toBe('00:00')
  })
})
