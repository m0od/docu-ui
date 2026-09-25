import { createFileRoute } from '@tanstack/react-router'
import { EnvFileView } from '@/features/env-files/env-file-view'

export const Route = createFileRoute('/_authenticated/env-files/$name')({
  component: EnvFileView,
})
