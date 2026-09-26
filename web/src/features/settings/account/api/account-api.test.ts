import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  changePassword,
  disableTotp,
  enableTotp,
  fetchAccount,
  fetchAccountTotpSecret,
} from './account-api'

function mockFetch(responseBody: unknown) {
  return vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(Response.json(responseBody))
}

describe('account-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('reads the account and a new TOTP secret', async () => {
    mockFetch({ username: 'admin', totpEnabled: true })
    await expect(fetchAccount()).resolves.toEqual({
      username: 'admin',
      totpEnabled: true,
    })
    mockFetch({ secret: 'JBSWY3DP' })
    await expect(fetchAccountTotpSecret()).resolves.toBe('JBSWY3DP')
  })

  it('sends the current password with every change', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockImplementation(async () => new Response(null, { status: 204 }))
    await changePassword('old-password', 'a-new-long-password')
    await enableTotp({ currentPassword: 'pw', secret: 'S', code: '123456' })
    await disableTotp({ currentPassword: 'pw', code: '654321' })

    expect(
      fetchSpy.mock.calls.map(([url, init]) => [url, init?.method, init?.body])
    ).toEqual([
      [
        'api/account/password',
        'PUT',
        '{"currentPassword":"old-password","newPassword":"a-new-long-password"}',
      ],
      [
        'api/account/totp',
        'POST',
        '{"currentPassword":"pw","secret":"S","code":"123456"}',
      ],
      [
        'api/account/totp/disable',
        'POST',
        '{"currentPassword":"pw","code":"654321"}',
      ],
    ])
  })
})
