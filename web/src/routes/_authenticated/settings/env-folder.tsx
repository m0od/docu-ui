import { createFileRoute } from '@tanstack/react-router'
import { SettingsEnvFolder } from '@/features/settings/env-folder'

export const Route = createFileRoute('/_authenticated/settings/env-folder')({
  component: SettingsEnvFolder,
})
