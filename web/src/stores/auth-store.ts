import { create } from 'zustand'

// The session is an HttpOnly cookie the server checks; this store only holds
// who is signed in so the UI can show it. It is filled from GET /api/auth/me.
interface AuthUser {
  username: string
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
