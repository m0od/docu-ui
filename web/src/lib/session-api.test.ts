import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchCurrentUser, signIn, signOut } from './session-api'

function mockFetch(response: Response) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(response)
}

const credentials = { username: 'admin', password: 'secret', totpCode: '' }

describe('session-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('signIn returns the username on success', async () => {
    mockFetch(Response.json({ username: 'admin' }))

    await expect(signIn(credentials)).resolves.toEqual({
      status: 'signed-in',
      username: 'admin',
    })
  })

  // Correct password but the account has two-factor authentication:
  // the form must ask for the code, not show a failure.
  it('signIn reports that a TOTP code is needed', async () => {
    mockFetch(
      Response.json(
        { error: 'enter the code', totpRequired: true },
        { status: 401 }
      )
    )

    await expect(signIn(credentials)).resolves.toEqual({
      status: 'totp-required',
      message: 'enter the code',
    })
  })

  it('signIn throws on a real failure', async () => {
    mockFetch(
      Response.json({ error: 'invalid username or password' }, { status: 401 })
    )

    await expect(signIn(credentials)).rejects.toThrow(
      'invalid username or password'
    )
  })

  it('signIn rethrows network errors', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(
      new TypeError('offline')
    )

    await expect(signIn(credentials)).rejects.toThrow('offline')
  })

  it('fetchCurrentUser returns the signed-in username', async () => {
    mockFetch(Response.json({ username: 'admin' }))

    await expect(fetchCurrentUser()).resolves.toBe('admin')
  })

  // No session is normal (first visit, expired): the guard redirects, no crash.
  it('fetchCurrentUser returns null without a session', async () => {
    mockFetch(Response.json({ error: 'not signed in' }, { status: 401 }))

    await expect(fetchCurrentUser()).resolves.toBeNull()
  })

  it('fetchCurrentUser throws on server errors', async () => {
    mockFetch(Response.json({ error: 'database down' }, { status: 500 }))

    await expect(fetchCurrentUser()).rejects.toThrow('database down')
  })

  it('fetchCurrentUser rethrows network errors', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(
      new TypeError('offline')
    )

    await expect(fetchCurrentUser()).rejects.toThrow('offline')
  })

  it('signOut posts to the logout endpoint', async () => {
    const fetchSpy = mockFetch(new Response(null, { status: 204 }))

    await signOut()

    expect(fetchSpy).toHaveBeenCalledWith(
      'api/auth/logout',
      expect.objectContaining({ method: 'POST' })
    )
  })
})
