import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createFirstAccount,
  fetchTotpSecret,
  isSetupRequired,
  otpauthUri,
  skipSignIn,
} from './setup-api'

describe('setup-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('reads whether setup is needed and a TOTP secret', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(Response.json({ required: true }))
      .mockResolvedValueOnce(Response.json({ secret: 'JBSWY3DP' }))

    await expect(isSetupRequired()).resolves.toBe(true)
    await expect(fetchTotpSecret()).resolves.toBe('JBSWY3DP')
  })

  // Skipping sign-in sends only the token: no account fields that the server would ignore.
  it('creates the account or skips sign-in', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockImplementation(async () => Response.json({}, { status: 201 }))
    await createFirstAccount({
      setupToken: 't',
      username: 'admin',
      password: 'a-long-password',
      totpSecret: '',
      totpCode: '',
    })
    await skipSignIn('t')

    expect(fetchSpy.mock.calls.map(([url, init]) => [url, init?.body])).toEqual(
      [
        [
          'api/setup',
          '{"setupToken":"t","username":"admin","password":"a-long-password","totpSecret":"","totpCode":""}',
        ],
        ['api/setup', '{"setupToken":"t","skipSignIn":true}'],
      ]
    )
  })

  it('builds the otpauth link, falling back to admin for an empty username', () => {
    expect(otpauthUri('', 'S')).toBe(
      'otpauth://totp/Docu-UI%3Aadmin?secret=S&issuer=Docu-UI'
    )
  })
})
