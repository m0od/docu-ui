import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  applyEnvFile,
  fetchApplyState,
  fetchDocoCD,
  fetchSharedWebhook,
  saveApplyTarget,
  saveDocoCD,
  saveSharedWebhook,
} from './apply-api'

function mockFetch(responseBody: unknown) {
  return vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(Response.json(responseBody))
}

describe('apply-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('reads and saves the Doco-CD settings', async () => {
    const settings = { url: 'http://doco-cd', hasApiKey: true }
    mockFetch(settings)
    await expect(fetchDocoCD()).resolves.toEqual(settings)

    const fetchSpy = mockFetch(settings)
    await saveDocoCD('http://doco-cd', '')
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/settings/doco-cd',
      expect.objectContaining({
        method: 'PUT',
        body: '{"url":"http://doco-cd","apiKey":""}',
      })
    )
  })

  it('reads and saves the shared webhook', async () => {
    const view = {
      url: 'https://ci.example/hook',
      hasSecret: true,
      headerName: '',
      hasHeaderValue: false,
    }
    mockFetch(view)
    await expect(fetchSharedWebhook()).resolves.toEqual(view)

    const fetchSpy = mockFetch(view)
    await saveSharedWebhook({
      url: 'https://ci.example/hook',
      secret: 's',
      headerName: '',
      headerValue: '',
    })
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/settings/webhook',
      expect.objectContaining({
        method: 'PUT',
        body: '{"url":"https://ci.example/hook","secret":"s","headerName":"","headerValue":""}',
      })
    )
  })

  // File names come from the file system; escape them like every other file URL.
  it('reads, saves and applies the target of a file', async () => {
    const applyState = {
      target: {
        adapter: 'doco-cd',
        project: 'p',
        services: ['s'],
        webhook: {
          url: '',
          hasSecret: false,
          headerName: '',
          hasHeaderValue: false,
        },
      },
      appliedVersion: 'v1',
    }
    const readSpy = mockFetch(applyState)
    await expect(fetchApplyState('a b.env')).resolves.toEqual(applyState)
    expect(readSpy).toHaveBeenCalledWith(
      'api/env-files/a%20b.env/apply-target',
      undefined
    )

    const saveSpy = mockFetch(applyState)
    const noWebhook = { url: '', secret: '', headerName: '', headerValue: '' }
    await saveApplyTarget('a.env', {
      adapter: 'webhook',
      project: '',
      services: [],
      webhook: noWebhook,
    })
    expect(saveSpy).toHaveBeenCalledWith(
      'api/env-files/a.env/apply-target',
      expect.objectContaining({
        method: 'PUT',
        body: '{"adapter":"webhook","project":"","services":[],"webhook":{"url":"","secret":"","headerName":"","headerValue":""}}',
      })
    )

    const applySpy = mockFetch(applyState)
    await applyEnvFile('a.env', 'v2')
    expect(applySpy).toHaveBeenCalledWith(
      'api/env-files/a.env/apply',
      expect.objectContaining({ method: 'POST', body: '{"version":"v2"}' })
    )
  })
})
