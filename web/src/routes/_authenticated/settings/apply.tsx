import { createFileRoute } from '@tanstack/react-router'
import { SettingsApply } from '@/features/settings/apply'

export const Route = createFileRoute('/_authenticated/settings/apply')({
  component: SettingsApply,
})
