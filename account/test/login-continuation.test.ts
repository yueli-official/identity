import { describe, expect, it } from 'vitest'

import { existingSessionReturnTo } from '../app/utils/returnTo'

describe('existingSessionReturnTo', () => {
  it('continues an ordinary product OIDC authorization', () => {
    const target = '/oauth2/authorize?response_type=code&client_id=blog-main-web&redirect_uri=http%3A%2F%2F192.168.5.7%3A3002%2Fauth%2Fcallback'
    expect(existingSessionReturnTo(target)).toBe(target)
  })

  it('keeps explicit re-authentication on the login page', () => {
    expect(existingSessionReturnTo('/oauth2/authorize?prompt=login')).toBeNull()
    expect(existingSessionReturnTo('/oauth2/authorize?max_age=0')).toBeNull()
    expect(existingSessionReturnTo('/oauth2/authorize', true)).toBeNull()
  })

  it('retains safe relative-path enforcement', () => {
    expect(existingSessionReturnTo('https://evil.example/steal')).toBe('/')
  })
})
