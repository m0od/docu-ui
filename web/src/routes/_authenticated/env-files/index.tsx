import { createFileRoute } from '@tanstack/react-router'
import { EnvFiles } from '@/features/env-files'

export const Route = createFileRoute('/_authenticated/env-files/')({
  component: EnvFiles,
})
