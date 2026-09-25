import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { readEnvFile } from './api/env-files-api'
import { EnvFileView } from './env-file-view'

vi.mock('./api/env-files-api', () => ({
  readEnvFile: vi.fn(),
  revealEnvValue: vi.fn(),
}))
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useParams: () => ({ name: 'keycloak.env' }),
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}))
vi.mock('./components/env-variables-editor', () => ({
  EnvVariablesEditor: ({ variables }: { variables: { key: string }[] }) => (
    <p>variables: {variables.map((variable) => variable.key).join(',')}</p>
  ),
}))
vi.mock('./components/env-text-editor', () => ({
  EnvTextEditor: ({ onClose }: { onClose: () => void }) => (
    <button onClick={onClose}>close text editor</button>
  ),
}))
vi.mock('./components/env-history', () => ({
  EnvHistory: ({ onClose }: { onClose: () => void }) => (
    <button onClick={onClose}>close history</button>
  ),
}))
vi.mock('@/components/layout/header', () => ({
  Header: () => null,
}))
vi.mock('@/components/layout/main', () => ({
  Main: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}))

describe('EnvFileView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('opens the variable editor for the file named in the URL', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      version: 'version-1',
      variables: [{ key: 'KC_DB', line: 1, overridden: false }],
      invalidLines: [],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect
      .element(screen.getByText('variables: KC_DB'))
      .toBeInTheDocument()
    expect(readEnvFile).toHaveBeenCalledWith('keycloak.env')
    await expect.element(screen.getByRole('alert')).not.toBeInTheDocument()
  })

  // The text editor shows every secret; the user must agree first.
  it('warns before switching to the text editor, and can go back', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      version: 'version-1',
      variables: [],
      invalidLines: [],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await userEvent.click(screen.getByRole('button', { name: 'Edit as text' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('including passwords')
    await expect
      .element(screen.getByRole('button', { name: 'close text editor' }))
      .not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Show and edit' }))

    await userEvent.click(
      screen.getByRole('button', { name: 'close text editor' })
    )
    await expect.element(screen.getByText('variables:')).toBeInTheDocument()
  })

  // Old versions and the diff show secrets too; same warning, and it can be declined.
  it('warns before showing the history, and can go back', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      version: 'version-1',
      variables: [],
      invalidLines: [],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await userEvent.click(screen.getByRole('button', { name: 'History' }))
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'close history' }))
      .not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'History' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('including passwords')
    await userEvent.click(screen.getByRole('button', { name: 'Show history' }))
    await userEvent.click(screen.getByRole('button', { name: 'close history' }))
    await expect.element(screen.getByText('variables:')).toBeInTheDocument()
  })

  // Compose refuses such a file on the next deploy; warn before that happens.
  it('warns about lines Compose cannot read', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      version: 'version-1',
      variables: [],
      invalidLines: [3, 7],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('Lines 3, 7 are neither')
  })

  it('uses the singular for one broken line', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      version: 'version-1',
      variables: [],
      invalidLines: [4],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('Line 4 is neither')
  })

  it('shows the server error', async () => {
    vi.mocked(readEnvFile).mockRejectedValueOnce(
      new Error('env file not found')
    )
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('env file not found')
  })
})
