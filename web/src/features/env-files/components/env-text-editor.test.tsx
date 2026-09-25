import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { ApiError } from '@/lib/api-client'
import { readEnvContent, saveEnvContent } from '../api/env-files-api'
import { EnvTextEditor } from './env-text-editor'

vi.mock('../api/env-files-api', () => ({
  readEnvContent: vi.fn(),
  saveEnvContent: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))

const onClose = vi.fn()

function renderEditor() {
  return render(
    withQueryClient(<EnvTextEditor fileName='keycloak.env' onClose={onClose} />)
  )
}

describe('EnvTextEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the diff and saves from the version it loaded', async () => {
    vi.mocked(readEnvContent).mockResolvedValueOnce({
      content: 'A=1\n',
      version: 'version-1',
    })
    vi.mocked(saveEnvContent).mockResolvedValueOnce()
    const screen = await renderEditor()
    const textArea = screen.getByLabelText('Content of keycloak.env')
    await expect.element(textArea).toHaveValue('A=1\n')
    // Nothing to save until something changed.
    await expect
      .element(screen.getByRole('button', { name: 'Review and save' }))
      .toBeDisabled()

    await userEvent.fill(textArea, 'A=2\n')
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )
    await expect.element(screen.getByText('− A=1')).toBeInTheDocument()
    await expect.element(screen.getByText('+ A=2')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await vi.waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(saveEnvContent).toHaveBeenCalledWith(
      'keycloak.env',
      'version-1',
      'A=2\n'
    )
  })

  it('cancels without saving', async () => {
    vi.mocked(readEnvContent).mockResolvedValueOnce({
      content: '',
      version: 'v',
    })
    const screen = await renderEditor()

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onClose).toHaveBeenCalledOnce()
    expect(saveEnvContent).not.toHaveBeenCalled()
  })

  it('reloads the saved file after a conflict', async () => {
    vi.mocked(readEnvContent)
      .mockResolvedValueOnce({ content: 'A=1\n', version: 'version-1' })
      .mockResolvedValueOnce({ content: 'A=9\n', version: 'version-2' })
    vi.mocked(saveEnvContent).mockRejectedValueOnce(
      new ApiError(409, {
        error: 'the file changed since you opened it: reload and try again',
      })
    )
    const screen = await renderEditor()
    await userEvent.fill(screen.getByLabelText(/Content of/), 'A=2\n')
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('copy them first')
    await userEvent.click(screen.getByRole('button', { name: 'Reload file' }))

    await expect
      .element(screen.getByLabelText(/Content of/))
      .toHaveValue('A=9\n')
  })

  it('shows other save errors without a reload link', async () => {
    vi.mocked(readEnvContent).mockResolvedValueOnce({
      content: 'A=1\n',
      version: 'v',
    })
    vi.mocked(saveEnvContent).mockRejectedValueOnce(
      new Error('permission denied')
    )
    const screen = await renderEditor()
    await userEvent.fill(screen.getByLabelText(/Content of/), 'A=2\n')
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('permission denied')
    await expect
      .element(screen.getByRole('button', { name: 'Reload file' }))
      .not.toBeInTheDocument()
  })

  it('shows a load error', async () => {
    vi.mocked(readEnvContent).mockRejectedValueOnce(
      new Error('env file not found')
    )
    const screen = await renderEditor()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('env file not found')
  })
})
