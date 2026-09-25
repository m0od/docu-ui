import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { ApiError } from '@/lib/api-client'
import {
  listEnvHistory,
  readEnvContent,
  readEnvHistory,
  restoreEnvHistory,
} from '../api/env-files-api'
import { EnvHistory } from './env-history'

vi.mock('../api/env-files-api', () => ({
  listEnvHistory: vi.fn(),
  readEnvContent: vi.fn(),
  readEnvHistory: vi.fn(),
  restoreEnvHistory: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))

const onClose = vi.fn()
const olderEntry = {
  id: 'older',
  savedAt: '2026-09-24T08:00:00Z',
  savedBy: 'admin',
}
const newerEntry = {
  id: 'newer',
  savedAt: '2026-09-25T08:00:00Z',
  savedBy: 'ops.user',
}

function renderHistory() {
  return render(
    withQueryClient(<EnvHistory fileName='keycloak.env' onClose={onClose} />)
  )
}

// Opens the history on newerEntry, holding oldContent, while the file holds A=2.
async function openNewerEntry(oldContent: string) {
  vi.mocked(listEnvHistory).mockResolvedValueOnce([newerEntry, olderEntry])
  vi.mocked(readEnvHistory).mockResolvedValueOnce(oldContent)
  vi.mocked(readEnvContent).mockResolvedValue({
    content: 'A=2\n',
    version: 'version-2',
  })
  const screen = await renderHistory()
  await userEvent.click(screen.getByRole('button', { name: /ops\.user/ }))
  return screen
}

describe('EnvHistory', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('lists versions with who replaced them, and loads none until picked', async () => {
    vi.mocked(listEnvHistory).mockResolvedValueOnce([newerEntry, olderEntry])
    const screen = await renderHistory()

    await expect
      .element(screen.getByRole('button', { name: /before ops\.user saved/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /before admin saved/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Pick a version to compare'))
      .toBeInTheDocument()
    // Old versions hold secrets: nothing is fetched until the user asks.
    expect(readEnvHistory).not.toHaveBeenCalled()
    expect(listEnvHistory).toHaveBeenCalledWith('keycloak.env')
  })

  it('says so when the file was never saved here', async () => {
    vi.mocked(listEnvHistory).mockResolvedValueOnce([])
    const screen = await renderHistory()

    await expect
      .element(screen.getByText(/No versions yet/))
      .toBeInTheDocument()
  })

  it('shows why the history cannot be read', async () => {
    vi.mocked(listEnvHistory).mockRejectedValueOnce(new Error('disk gone'))
    const screen = await renderHistory()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('disk gone')
  })

  it('goes back to the variables', async () => {
    vi.mocked(listEnvHistory).mockResolvedValueOnce([])
    const screen = await renderHistory()

    await userEvent.click(
      screen.getByRole('button', { name: 'Back to variables' })
    )
    expect(onClose).toHaveBeenCalledOnce()
  })

  // The diff reads as "what restore does to the file now": current lines out, old lines in.
  it('diffs the picked version against the current file and restores it', async () => {
    vi.mocked(restoreEnvHistory).mockResolvedValueOnce()
    const screen = await openNewerEntry('A=1\n')

    await expect
      .element(screen.getByRole('button', { name: /ops\.user/ }))
      .toHaveAttribute('aria-pressed', 'true')
    await expect.element(screen.getByText('− A=2')).toBeInTheDocument()
    await expect.element(screen.getByText('+ A=1')).toBeInTheDocument()
    expect(readEnvHistory).toHaveBeenCalledWith('keycloak.env', 'newer')

    await userEvent.click(
      screen.getByRole('button', { name: 'Restore this version' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

    await vi.waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    // Each read of an old version is an audit line; a restore must not add one.
    expect(readEnvHistory).toHaveBeenCalledOnce()
    // Sent with the version it was compared to, so a change made meanwhile is not lost.
    expect(restoreEnvHistory).toHaveBeenCalledWith(
      'keycloak.env',
      'newer',
      'version-2'
    )
  })

  it('offers no restore when the version equals the current file', async () => {
    const screen = await openNewerEntry('A=2\n')

    await expect
      .element(
        screen.getByText('This version is the same as the current file.')
      )
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Restore this version' }))
      .toBeDisabled()
  })

  it('shows why the old version cannot be read', async () => {
    vi.mocked(listEnvHistory).mockResolvedValueOnce([newerEntry])
    vi.mocked(readEnvHistory).mockRejectedValueOnce(
      new Error('history version not found')
    )
    vi.mocked(readEnvContent).mockResolvedValueOnce({
      content: '',
      version: 'v',
    })
    const screen = await renderHistory()
    await userEvent.click(screen.getByRole('button', { name: /ops\.user/ }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('history version not found')
  })

  it('shows why the current file cannot be read', async () => {
    vi.mocked(listEnvHistory).mockResolvedValueOnce([newerEntry])
    vi.mocked(readEnvHistory).mockResolvedValueOnce('A=1\n')
    vi.mocked(readEnvContent).mockRejectedValueOnce(
      new Error('env file not found')
    )
    const screen = await renderHistory()
    await userEvent.click(screen.getByRole('button', { name: /ops\.user/ }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('env file not found')
  })

  // Someone saved the file after the diff was shown: reload the diff, never overwrite blindly.
  it('reloads the current file after a conflict', async () => {
    vi.mocked(restoreEnvHistory).mockRejectedValueOnce(
      new ApiError(409, { error: 'the file changed since you opened it' })
    )
    const screen = await openNewerEntry('A=1\n')
    await userEvent.click(
      screen.getByRole('button', { name: 'Restore this version' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Restore' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('the file changed since you opened it')

    await userEvent.click(screen.getByRole('button', { name: 'Reload file' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .not.toBeInTheDocument()
    expect(readEnvContent).toHaveBeenCalledTimes(2)
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows other restore errors without a reload', async () => {
    vi.mocked(restoreEnvHistory).mockRejectedValueOnce(
      new ApiError(500, {
        error: 'cannot save the env file: permission denied',
      })
    )
    const screen = await openNewerEntry('A=1\n')
    await userEvent.click(
      screen.getByRole('button', { name: 'Restore this version' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('permission denied')
    await expect
      .element(screen.getByRole('button', { name: 'Reload file' }))
      .not.toBeInTheDocument()
  })
})
