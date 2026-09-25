import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { isSetupRequired } from '@/features/auth/setup/api/setup-api'

export const Route = createFileRoute('/_authenticated')({
  // A fresh install has no account to sign in with, so send the visitor to setup first.
  beforeLoad: async () => {
    if (await isSetupRequired()) {
      throw redirect({ to: '/setup', replace: true })
    }
  },
  component: AuthenticatedLayout,
})
