// Calls to the first-run setup API. URLs are relative so they resolve against
// <base href>, which keeps them working when Docu-UI is served under a sub-path.

export type CreateFirstAccountInput = {
  setupToken: string
  username: string
  password: string
  // Empty when the admin skips two-factor authentication.
  totpSecret: string
  totpCode: string
}

async function requestJson<ResponseBody>(
  url: string,
  init?: RequestInit
): Promise<ResponseBody> {
  const response = await fetch(url, init)
  const responseBody = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(
      responseBody.error ?? `Request failed (HTTP ${response.status})`
    )
  }
  return responseBody as ResponseBody
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
  await requestJson('api/setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

// otpauthUri builds the link authenticator apps read from the QR code.
export function otpauthUri(username: string, secret: string): string {
  const issuer = 'Docu-UI'
  const label = encodeURIComponent(`${issuer}:${username || 'admin'}`)
  return `otpauth://totp/${label}?secret=${secret}&issuer=${encodeURIComponent(issuer)}`
}
