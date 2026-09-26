// Calls for applying saved env files to running containers through Doco-CD.
import { postJson, requestJson, sendJson } from '@/lib/api-client'

type DocoCDSettings = {
  url: string
  // The key itself never comes back from the server.
  hasApiKey: boolean
}

export type ApplyAdapter = 'doco-cd' | 'compose' | 'webhook'

// A webhook as the server shows it: whether secrets are set, never their values.
export type WebhookView = {
  // Empty in a file's target: the file uses the shared webhook.
  url: string
  hasSecret: boolean
  headerName: string
  hasHeaderValue: boolean
}

// A webhook as the page sends it; an empty secret or header value keeps the saved one.
export type WebhookInput = {
  url: string
  secret: string
  headerName: string
  headerValue: string
}

export type ApplyTarget = {
  adapter: ApplyAdapter
  project: string
  services: string[]
  webhook: WebhookView
}

type ApplyTargetInput = Omit<ApplyTarget, 'webhook'> & { webhook: WebhookInput }

type ApplyState = {
  // null until the user says which project and services use this file.
  target: ApplyTarget | null
  // The file version applied last; a different current version is "not applied yet".
  appliedVersion: string
}

export function fetchDocoCD(): Promise<DocoCDSettings> {
  return requestJson('api/settings/doco-cd')
}

// An empty apiKey keeps the saved one.
export function saveDocoCD(
  url: string,
  apiKey: string
): Promise<DocoCDSettings> {
  return sendJson('PUT', 'api/settings/doco-cd', { url, apiKey })
}

export function fetchSharedWebhook(): Promise<WebhookView> {
  return requestJson('api/settings/webhook')
}

export function saveSharedWebhook(webhook: WebhookInput): Promise<WebhookView> {
  return sendJson('PUT', 'api/settings/webhook', webhook)
}

function applyUrl(fileName: string, path: string): string {
  return `api/env-files/${encodeURIComponent(fileName)}/${path}`
}

export function fetchApplyState(fileName: string): Promise<ApplyState> {
  return requestJson(applyUrl(fileName, 'apply-target'))
}

export function saveApplyTarget(
  fileName: string,
  target: ApplyTargetInput
): Promise<ApplyState> {
  return sendJson('PUT', applyUrl(fileName, 'apply-target'), target)
}

// applyEnvFile recreates the target's services; version is the one the user looked at.
export function applyEnvFile(
  fileName: string,
  version: string
): Promise<ApplyState> {
  return postJson(applyUrl(fileName, 'apply'), { version })
}
