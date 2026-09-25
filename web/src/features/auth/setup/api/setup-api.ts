// Calls to the first-run setup API.
import { postJson, requestJson } from '@/lib/api-client'

export type CreateFirstAccountInput = {
  setupToken: string
  username: string
  password: string
  // Empty when the admin skips two-factor authentication.
  totpSecret: string
  totpCode: string
}

export async function isSetupRequired(): Promise<boolean> {
  const status = await requestJson<{ required: boolean }>('api/setup')
  return status.required
}

export async function fetchTotpSecret(): Promise<string> {
  const response = await requestJson<{ secret: string }>(
    'api/setup/totp-secret'
  )
  return response.secret
}

export async function createFirstAccount(
  input: CreateFirstAccountInput
): Promise<void> {
  await postJson('api/setup', input)
}

// otpauthUri builds the link authenticator apps read from the QR code.
export function otpauthUri(username: string, secret: string): string {
  const issuer = 'Docu-UI'
  const label = encodeURIComponent(`${issuer}:${username || 'admin'}`)
  return `otpauth://totp/${label}?secret=${secret}&issuer=${encodeURIComponent(issuer)}`
}
