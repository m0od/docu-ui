// Calls to the account API of the signed-in admin.
import { postJson, requestJson, sendJson } from '@/lib/api-client'

type Account = {
  username: string
  totpEnabled: boolean
}

export function fetchAccount(): Promise<Account> {
  return requestJson<Account>('api/account')
}

export async function changePassword(
  currentPassword: string,
  newPassword: string
): Promise<void> {
  await sendJson('PUT', 'api/account/password', {
    currentPassword,
    newPassword,
  })
}

export async function fetchAccountTotpSecret(): Promise<string> {
  const response = await requestJson<{ secret: string }>(
    'api/account/totp-secret'
  )
  return response.secret
}

export async function enableTotp(input: {
  currentPassword: string
  secret: string
  code: string
}): Promise<void> {
  await postJson('api/account/totp', input)
}

export async function disableTotp(input: {
  currentPassword: string
  code: string
}): Promise<void> {
  await postJson('api/account/totp/disable', input)
}
