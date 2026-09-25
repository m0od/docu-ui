// Calls to the env files API. Values are never part of a file listing;
// each one is fetched on its own when the user clicks to reveal it.
import { putJson, requestJson } from '@/lib/api-client'

export type EnvVariable = {
  key: string
  line: number
  // A later line sets the same key; Compose uses that one.
  overridden: boolean
}

type EnvFile = {
  name: string
  variables: EnvVariable[]
  // Lines that are neither comments nor KEY=value.
  invalidLines: number[]
}

export async function fetchEnvFolder(): Promise<string> {
  const settings = await requestJson<{ folder: string }>(
    'api/settings/env-folder'
  )
  return settings.folder
}

export async function saveEnvFolder(folder: string): Promise<void> {
  await putJson('api/settings/env-folder', { folder })
}

export async function listEnvFiles(): Promise<string[]> {
  const listing = await requestJson<{ files: string[] }>('api/env-files')
  return listing.files
}

export function readEnvFile(fileName: string): Promise<EnvFile> {
  return requestJson<EnvFile>(`api/env-files/${encodeURIComponent(fileName)}`)
}

export async function revealEnvValue(
  fileName: string,
  key: string
): Promise<string> {
  const variable = await requestJson<{ value: string }>(
    `api/env-files/${encodeURIComponent(fileName)}/variables/${encodeURIComponent(key)}`
  )
  return variable.value
}
