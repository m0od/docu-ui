// Calls to the env files API. Values are never part of a file listing;
// each one is fetched on its own when the user clicks to reveal it.
import { requestJson, sendJson } from '@/lib/api-client'

export type EnvVariable = {
  key: string
  line: number
  // A later line sets the same key; Compose uses that one.
  overridden: boolean
}

type EnvFile = {
  name: string
  // Sent back on save; the server refuses it if the file changed meanwhile.
  version: string
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
  await sendJson('PUT', 'api/settings/env-folder', { folder })
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

// EnvChange sets key to value, or removes every line of key when value is null.
export type EnvChange = { key: string; value: string | null }

function envFileUrl(fileName: string, path: string): string {
  return `api/env-files/${encodeURIComponent(fileName)}/${path}`
}

export async function changeEnvVariables(
  fileName: string,
  baseVersion: string,
  changes: EnvChange[]
): Promise<void> {
  await sendJson('PATCH', envFileUrl(fileName, 'variables'), {
    baseVersion,
    changes,
  })
}

// readEnvContent returns the whole file with every value in clear.
export function readEnvContent(
  fileName: string
): Promise<{ content: string; version: string }> {
  return requestJson(envFileUrl(fileName, 'content'))
}

export async function saveEnvContent(
  fileName: string,
  baseVersion: string,
  content: string
): Promise<void> {
  await sendJson('PUT', envFileUrl(fileName, 'content'), {
    baseVersion,
    content,
  })
}
