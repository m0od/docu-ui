import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { saveEnvFolder } from '../api/env-files-api'
import { EnvFolderForm } from './env-folder-form'

vi.mock('../api/env-files-api', () => ({ saveEnvFolder: vi.fn() }))

describe('EnvFolderForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the saved folder and only enables Save after a change', async () => {
    const screen = await render(
      withQueryClient(<EnvFolderForm savedFolder='/host/env' />)
    )

    await expect
      .element(screen.getByLabelText('Env folder'))
      .toHaveValue('/host/env')
    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeDisabled()
  })

  it('saves the trimmed path', async () => {
    vi.mocked(saveEnvFolder).mockResolvedValueOnce()
    const screen = await render(
      withQueryClient(<EnvFolderForm savedFolder='' />)
    )

    await userEvent.fill(screen.getByLabelText('Env folder'), ' /host/env ')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await vi.waitFor(() =>
      expect(vi.mocked(saveEnvFolder).mock.calls[0][0]).toBe('/host/env')
    )
  })

  // No double submit while the server is still checking the folder.
  it('disables Save while saving', async () => {
    vi.mocked(saveEnvFolder).mockReturnValueOnce(new Promise(() => {}))
    const screen = await render(
      withQueryClient(<EnvFolderForm savedFolder='' />)
    )

    await userEvent.fill(screen.getByLabelText('Env folder'), '/host/env')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeDisabled()
  })

  // The server refuses paths that are not mounted; the user must see why.
  it('shows why the server refused the folder', async () => {
    vi.mocked(saveEnvFolder).mockRejectedValueOnce(
      new Error('cannot read folder: no such file or directory')
    )
    const screen = await render(
      withQueryClient(<EnvFolderForm savedFolder='' />)
    )

    await userEvent.fill(screen.getByLabelText('Env folder'), '/typo')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('no such file or directory')
  })
})
