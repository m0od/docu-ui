import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, postJson, requestJson } from './api-client'

function mockFetch(response: Response) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(response)
}

describe('api-client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns the parsed body on success', async () => {
    mockFetch(Response.json({ required: true }))

    await expect(requestJson('api/setup')).resolves.toEqual({ required: true })
  })

  // The sign-in form needs the extra fields, not only the message.
  it('keeps status and the whole error body', async () => {
    mockFetch(
      Response.json(
        { error: 'enter the code', totpRequired: true },
        { status: 401 }
      )
    )

    const error: ApiError = await requestJson<never>('api/auth/login').catch(
      (caught: ApiError) => caught
    )

    expect(error).toBeInstanceOf(ApiError)
    expect(error.message).toBe('enter the code')
    expect(error.status).toBe(401)
    expect(error.responseBody.totpRequired).toBe(true)
  })

  // A gateway error page (HTML) has no JSON message to show.
  it('falls back to the HTTP status when the error body is not JSON', async () => {
    mockFetch(new Response('<html>Bad Gateway</html>', { status: 502 }))

    await expect(requestJson('api/setup')).rejects.toThrow(
      'Request failed (HTTP 502)'
    )
  })

  it('accepts an empty success body such as 204 No Content', async () => {
    mockFetch(new Response(null, { status: 204 }))

    await expect(postJson('api/auth/logout')).resolves.toEqual({})
  })

  // The server refuses POSTs that are not JSON (CSRF guard).
  it('posts JSON with the JSON content type', async () => {
    const fetchSpy = mockFetch(Response.json({}))

    await postJson('api/setup', { username: 'admin' })

    expect(fetchSpy).toHaveBeenCalledWith('api/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{"username":"admin"}',
    })
  })
})
