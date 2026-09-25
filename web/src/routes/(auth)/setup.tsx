import { createFileRoute, redirect } from '@tanstack/react-router'
import { Setup } from '@/features/auth/setup'
import { isSetupRequired } from '@/features/auth/setup/api/setup-api'

export const Route = createFileRoute('/(auth)/setup')({
  // Once the admin exists, the setup page is gone for good.
  beforeLoad: async () => {
    if (!(await isSetupRequired())) {
      throw redirect({ to: '/sign-in', replace: true })
    }
  },
  component: Setup,
})
