import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const readApp = (path: string) => readFileSync(resolve(process.cwd(), 'app', path), 'utf8')

describe('account security hardening', () => {
  it('uses the shared bounded toast region', () => {
    const app = readApp('app.vue')
    expect(app).toContain('FeedbackToastRegion')
    expect(app).toContain('<UApp :toaster="null">')
  })

  it('recovers recent-authentication requirements in place', () => {
    expect(readApp('components/TOTPManager.vue')).toContain('reauthentication.run')
    expect(readApp('components/PasskeyManager.vue')).toContain('reauthentication.run')
    expect(readApp('pages/security.vue')).toContain('AccountReauthenticationModal')
  })

  it('explains insecure WebAuthn origins separately from browser support', () => {
    const passkeys = readApp('components/PasskeyManager.vue')
    expect(passkeys).toContain('通行密钥需要 HTTPS 或 localhost')
    expect(passkeys).toContain('局域网 HTTP')
  })

  it('inherits the shared single-border field theme from one app config', () => {
    const config = readApp('app.config.ts')
    expect(config).toContain('createUiPreset({ primary: "teal" })')
    expect(config).not.toContain('account-field-border')
  })
})
