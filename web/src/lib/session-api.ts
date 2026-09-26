// Calls to the sign-in / session API. The session itself lives in an HttpOnly
// cookie the browser sends automatically, so no token is kept in JavaScript.
import { ApiError, postJson, requestJson } from '@/lib/api-client'

type SignInInput = {
  username: string
  password: string
  // Empty on the first attempt; filled after the server asks for it.
  totpCode: string
}

type SignInResult =
  | { status: 'signed-in'; username: string }
  | { status: 'totp-required'; message: string }

export async function signIn(input: SignInInput): Promise<SignInResult> {
  try {
    const account = await postJson<{ username: string }>(
      'api/auth/login',
      input
    )
    return { status: 'signed-in', username: account.username }
  } catch (error) {
    // A correct password on an account with two-factor authentication:
    // not a failure, the form asks for the code next.
    if (error instanceof ApiError && error.responseBody.totpRequired === true) {
      return { status: 'totp-required', message: error.message }
    }
    throw error
  }
}

// CurrentUser.signIn is false when Docu-UI runs without sign-in; username is then "anonymous".
type CurrentUser = { username: string; signIn: boolean }

// fetchCurrentUser returns who is using Docu-UI, or null when there is no
// valid session (never signed in, expired, or signed out elsewhere).
export async function fetchCurrentUser(): Promise<CurrentUser | null> {
  try {
    return await requestJson<CurrentUser>('api/auth/me')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return null
    }
    throw error
  }
}

export async function signOut(): Promise<void> {
  await postJson('api/auth/logout')
}
