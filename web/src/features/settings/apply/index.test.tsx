import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { SettingsApply } from './index'

vi.mock('@/features/env-files/components/doco-cd-form', () => ({
  DocoCDForm: () => <p>doco-cd settings</p>,
}))
vi.mock('@/features/env-files/components/shared-webhook-form', () => ({
  SharedWebhookForm: () => <p>shared webhook settings</p>,
}))

describe('SettingsApply', () => {
  // Both adapters that need settings are set up here; Docker Compose needs none.
  it('holds the Doco-CD and shared webhook settings', async () => {
    const screen = await render(<SettingsApply />)

    await expect
      .element(screen.getByText('doco-cd settings'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('shared webhook settings'))
      .toBeInTheDocument()
  })
})
