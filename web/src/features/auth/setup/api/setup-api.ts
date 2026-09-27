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

// createFirstAccount returns the recovery codes when TOTP is on, else an empty list.
export async function createFirstAccount(
  input: CreateFirstAccountInput
): Promise<string[]> {
  const response = await postJson<{ recoveryCodes?: string[] }>(
    'api/setup',
    input
  )
  return response.recoveryCodes ?? []
}

// skipSignIn finishes setup without an account: Docu-UI then opens without sign-in.
export async function skipSignIn(setupToken: string): Promise<void> {
  await postJson('api/setup', { setupToken, skipSignIn: true })
}

// otpauthUri builds the link authenticator apps read from the QR code.
export function otpauthUri(username: string, secret: string): string {
  const issuer = 'Docu-UI'
  const label = encodeURIComponent(`${issuer}:${username || 'admin'}`)
  return `otpauth://totp/${label}?secret=${secret}&issuer=${encodeURIComponent(issuer)}`
}
