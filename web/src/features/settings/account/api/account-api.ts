// Calls to the account API of the signed-in admin.
import { postJson, requestJson, sendJson } from '@/lib/api-client'

// Without sign-in there is no account, so nothing but the flag.
type Account =
  | { signIn: false }
  | { signIn: true; username: string; totpEnabled: boolean }

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

// turnOnSignIn creates the account while sign-in is off; this browser must then sign in too.
export async function turnOnSignIn(
  username: string,
  password: string
): Promise<void> {
  await postJson('api/account/sign-in', { username, password })
}

// turnOffSignIn deletes the account and every session. code is only checked when TOTP is on.
export async function turnOffSignIn(input: {
  currentPassword: string
  code: string
}): Promise<void> {
  await postJson('api/account/sign-in/disable', input)
}
