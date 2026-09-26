import { create } from 'zustand'

// The session is an HttpOnly cookie the server checks; this store only holds
// who is signed in so the UI can show it. It is filled from GET /api/auth/me.
// signIn is false when Docu-UI runs without sign-in: no sign-out, and a warning banner.
interface AuthUser {
  username: string
  signIn: boolean
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    setUser: (user) =>
      set((state) => ({ ...state, auth: { ...state.auth, user } })),
    reset: () =>
      set((state) => ({ ...state, auth: { ...state.auth, user: null } })),
  },
}))
