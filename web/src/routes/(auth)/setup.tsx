import { createFileRoute, redirect } from '@tanstack/react-router'
import { isSetupRequired } from '@/lib/setup-api'
import { Setup } from '@/features/setup'

export const Route = createFileRoute('/(auth)/setup')({
  // Once the admin exists, the setup page is gone for good.
  beforeLoad: async () => {
    if (!(await isSetupRequired())) {
      throw redirect({ to: '/sign-in', replace: true })
    }
  },
  component: Setup,
})
