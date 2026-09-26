import { createFileRoute, redirect } from '@tanstack/react-router'

// Env files are the app; there is no separate home page.
export const Route = createFileRoute('/_authenticated/')({
  beforeLoad: () => {
    throw redirect({ to: '/env-files' })
  },
})
