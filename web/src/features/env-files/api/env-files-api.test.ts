import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  changeEnvVariables,
  fetchEnvFolder,
  readEnvContent,
  saveEnvContent,
  listEnvFiles,
  readEnvFile,
  revealEnvValue,
  saveEnvFolder,
} from './env-files-api'

function mockFetch(responseBody: unknown) {
  return vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(Response.json(responseBody))
}

describe('env-files-api', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('reads and saves the env folder', async () => {
    mockFetch({ folder: '/host/env' })
    await expect(fetchEnvFolder()).resolves.toBe('/host/env')

    const fetchSpy = mockFetch({ folder: '/host/other' })
    await saveEnvFolder('/host/other')
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/settings/env-folder',
      expect.objectContaining({
        method: 'PUT',
        body: '{"folder":"/host/other"}',
      })
    )
  })

  it('lists file names', async () => {
    mockFetch({ folder: '/host/env', files: ['a.env', 'b.env'] })

    await expect(listEnvFiles()).resolves.toEqual(['a.env', 'b.env'])
  })

  it('reads one file', async () => {
    const envFile = {
      name: 'a.env',
      version: 'v1',
      variables: [],
      invalidLines: [],
    }
    const fetchSpy = mockFetch(envFile)

    await expect(readEnvFile('a.env')).resolves.toEqual(envFile)
    expect(fetchSpy).toHaveBeenCalledWith('api/env-files/a.env', undefined)
  })

  // Names come from the file system and keys from the file; a "/" or "?"
  // must stay part of the name, not change which URL is called.
  it('escapes the file name and key in the URL', async () => {
    const fetchSpy = mockFetch({ value: 'x' })

    await expect(revealEnvValue('a b.env', 'KEY/../?x')).resolves.toBe('x')
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/env-files/a%20b.env/variables/KEY%2F..%2F%3Fx',
      undefined
    )
  })

  it('sends variable changes with the version they were made on', async () => {
    const fetchSpy = mockFetch({ version: 'v2' })

    await changeEnvVariables('a.env', 'v1', [{ key: 'A', value: null }])

    expect(fetchSpy).toHaveBeenCalledWith(
      'api/env-files/a.env/variables',
      expect.objectContaining({
        method: 'PATCH',
        body: '{"baseVersion":"v1","changes":[{"key":"A","value":null}]}',
      })
    )
  })

  it('reads and saves the whole file', async () => {
    mockFetch({ content: 'A=1\n', version: 'v1' })
    await expect(readEnvContent('a.env')).resolves.toEqual({
      content: 'A=1\n',
      version: 'v1',
    })

    const fetchSpy = mockFetch({ version: 'v2' })
    await saveEnvContent('a.env', 'v1', 'A=2\n')
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/env-files/a.env/content',
      expect.objectContaining({
        method: 'PUT',
        body: '{"baseVersion":"v1","content":"A=2\\n"}',
      })
    )
  })
})
