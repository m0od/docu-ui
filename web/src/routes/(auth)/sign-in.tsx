import { z } from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { fetchCurrentUser } from '@/lib/session-api'
import { isSetupRequired } from '@/features/auth/setup/api/setup-api'
import { SignIn } from '@/features/auth/sign-in'

const searchSchema = z.object({
  redirect: z.string().optional(),
})

export const Route = createFileRoute('/(auth)/sign-in')({
  // Nobody can sign in before the first account exists, or while sign-in is off.
  beforeLoad: async () => {
    if (await isSetupRequired()) {
      throw redirect({ to: '/setup', replace: true })
    }
    if ((await fetchCurrentUser())?.signIn === false) {
      throw redirect({ to: '/', replace: true })
    }
  },
  component: SignIn,
  validateSearch: searchSchema,
})
