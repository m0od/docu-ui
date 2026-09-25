import { createFileRoute, redirect } from '@tanstack/react-router'
import { isSetupRequired } from '@/lib/setup-api'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'

export const Route = createFileRoute('/_authenticated')({
  // A fresh install has no account to sign in with, so send the visitor to setup first.
  beforeLoad: async () => {
    if (await isSetupRequired()) {
      throw redirect({ to: '/setup', replace: true })
    }
  },
  component: AuthenticatedLayout,
})
