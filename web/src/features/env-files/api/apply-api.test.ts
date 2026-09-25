import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  applyEnvFile,
  fetchApplyState,
  fetchDocoCD,
  saveApplyTarget,
  saveDocoCD,
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

  // File names come from the file system; escape them like every other file URL.
  it('reads, saves and applies the target of a file', async () => {
    const applyState = {
      target: { project: 'p', services: ['s'] },
      appliedVersion: 'v1',
    }
    const readSpy = mockFetch(applyState)
    await expect(fetchApplyState('a b.env')).resolves.toEqual(applyState)
    expect(readSpy).toHaveBeenCalledWith(
      'api/env-files/a%20b.env/apply-target',
      undefined
    )

    const saveSpy = mockFetch(applyState)
    await saveApplyTarget('a.env', { project: 'p', services: ['s'] })
    expect(saveSpy).toHaveBeenCalledWith(
      'api/env-files/a.env/apply-target',
      expect.objectContaining({
        method: 'PUT',
        body: '{"project":"p","services":["s"]}',
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
