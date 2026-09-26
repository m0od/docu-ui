import { beforeEach, describe, expect, it } from 'vitest'
import { useAuthStore } from './auth-store'

describe('useAuthStore', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
  })

  it('starts signed out', () => {
    expect(useAuthStore.getState().auth.user).toBeNull()
  })

  it('remembers who signed in so the sidebar can show the username', () => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })

    expect(useAuthStore.getState().auth.user).toEqual({
      username: 'admin',
      signIn: true,
    })
  })

  // After sign-out or a 401 the route guard must ask the server again
  // instead of trusting a stale user.
  it('reset forgets the user', () => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })

    useAuthStore.getState().auth.reset()

    expect(useAuthStore.getState().auth.user).toBeNull()
  })

  // The session cookie is HttpOnly; nothing about the session may be
  // readable (and so stealable) from JavaScript.
  it('does not write the session to a readable cookie', () => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })

    expect(document.cookie).not.toContain('admin')
  })
})
