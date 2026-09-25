import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
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

  it('lists the variables of the file named in the URL, values masked', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      variables: [{ key: 'KC_DB', line: 1, overridden: false }],
      invalidLines: [],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect.element(screen.getByText('KC_DB')).toBeInTheDocument()
    await expect.element(screen.getByText('••••••••')).toBeInTheDocument()
    expect(readEnvFile).toHaveBeenCalledWith('keycloak.env')
    await expect.element(screen.getByRole('alert')).not.toBeInTheDocument()
  })

  // Compose refuses such a file on the next deploy; warn before that happens.
  it('warns about lines Compose cannot read', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
      variables: [],
      invalidLines: [3, 7],
    })
    const screen = await render(withQueryClient(<EnvFileView />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('Lines 3, 7 are neither')
    await expect
      .element(screen.getByText('This file sets no variables.'))
      .toBeInTheDocument()
  })

  it('uses the singular for one broken line', async () => {
    vi.mocked(readEnvFile).mockResolvedValueOnce({
      name: 'keycloak.env',
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
