import { getApiFailure } from '@yueli/http-runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useApi } from '../app/composables/useApi'

const rawFetch = vi.fn()
const fetchStub = Object.assign(vi.fn(), { raw: rawFetch })

describe('useApi current Problem contract', () => {
  beforeEach(() => {
    rawFetch.mockReset()
    fetchStub.mockReset()
    vi.stubGlobal('$fetch', fetchStub)
    vi.stubGlobal('createError', (value: unknown) => value)
  })

  it('returns raw success DTOs', async () => {
    rawFetch.mockResolvedValue({
      status: 200,
      statusText: 'OK',
      headers: new Headers({ 'content-type': 'application/json' }),
      _data: { id: 'identity-1' },
    })

    await expect(useApi().call<{ id: string }>('/api/v1/session/me'))
      .resolves.toEqual({ id: 'identity-1' })
  })

  it('turns Problem responses into a stable ApiFailure with params and trace', async () => {
    rawFetch.mockResolvedValue({
      status: 400,
      statusText: 'Bad Request',
      headers: new Headers({
        'content-type': 'application/problem+json',
        'x-trace-id': 'trace-problem-test',
      }),
      _data: {
        type: 'https://errors.yueli.dev/problems/identity.weak_password',
        status: 400,
        code: 'identity.weak_password',
        params: { reason: 'blocklisted' },
        traceId: 'trace-problem-test',
      },
    })

    let caught: unknown
    try {
      await useApi().call('/api/v1/auth/password/change', {
        method: 'POST',
        body: { currentPassword: 'old', newPassword: 'new' },
      })
    } catch (error) {
      caught = error
    }

    expect(getApiFailure(caught)).toMatchObject({
      kind: 'remote',
      status: 400,
      code: 'identity.weak_password',
      params: { reason: 'blocklisted' },
      traceId: 'trace-problem-test',
    })
  })

  it('preserves the Account-owned step-up proof header', async () => {
    rawFetch.mockResolvedValue({
      status: 204,
      statusText: 'No Content',
      headers: new Headers(),
      _data: undefined,
    })

    await useApi().call('/api/v1/admin/users/identity-1', {
      method: 'DELETE',
      headers: { 'X-Step-Up-Proof': 'proof-value' },
    })

    expect(rawFetch).toHaveBeenCalledWith(
      '/api/v1/admin/users/identity-1',
      expect.objectContaining({
        headers: expect.objectContaining({ 'x-step-up-proof': 'proof-value' }),
      }),
    )
  })
})
