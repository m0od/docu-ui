import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { fetchEnvFolder } from '@/features/env-files/api/env-files-api'
import { SettingsEnvFolder } from './index'

vi.mock('@/features/env-files/api/env-files-api', () => ({
  fetchEnvFolder: vi.fn(),
  saveEnvFolder: vi.fn(),
}))

describe('SettingsEnvFolder', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  // The form starts from the saved folder, so saving without a change is not needed.
  it('shows the saved folder in the form', async () => {
    vi.mocked(fetchEnvFolder).mockResolvedValueOnce('/host/env')
    const screen = await render(withQueryClient(<SettingsEnvFolder />))

    await expect
      .element(screen.getByLabelText('Env folder'))
      .toHaveValue('/host/env')
  })

  it('shows why the folder cannot be read', async () => {
    vi.mocked(fetchEnvFolder).mockRejectedValueOnce(
      new Error('cannot read settings')
    )
    const screen = await render(withQueryClient(<SettingsEnvFolder />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read settings')
  })
})
