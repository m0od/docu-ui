import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { fetchCurrentUser } from '@/lib/session-api'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { isSetupRequired } from '@/features/auth/setup/api/setup-api'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    // A fresh install has no account to sign in with, so send the visitor to setup first.
    if (await isSetupRequired()) {
      throw redirect({ to: '/setup', replace: true })
    }
    const { auth } = useAuthStore.getState()
    if (auth.user) {
      return
    }
    const currentUser = await fetchCurrentUser()
    if (!currentUser) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
        replace: true,
      })
    }
    auth.setUser(currentUser)
  },
  component: AuthenticatedLayout,
})
