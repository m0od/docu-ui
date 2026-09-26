import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { fetchEnvFolder } from './api/env-files-api'
import { EnvFiles } from './index'

vi.mock('./api/env-files-api', () => ({ fetchEnvFolder: vi.fn() }))
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  Link: ({ to, ...linkProps }: React.ComponentProps<'a'> & { to: string }) => (
    <a href={to} {...linkProps} />
  ),
}))
vi.mock('./components/env-file-list', () => ({
  EnvFileList: () => <p>file list</p>,
}))
// The page header needs the app's layout providers; it is not what this test is about.
vi.mock('@/components/layout/header', () => ({
  Header: () => null,
}))
vi.mock('@/components/layout/main', () => ({
  Main: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}))

describe('EnvFiles page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // A fresh install: point to where the folder is chosen instead of showing an error or an empty list.
  it('asks for the folder when none is saved', async () => {
    vi.mocked(fetchEnvFolder).mockResolvedValueOnce('')
    const screen = await render(withQueryClient(<EnvFiles />))

    await expect
      .element(screen.getByRole('link', { name: 'Choose the folder' }))
      .toHaveAttribute('href', '/settings/env-folder')
    await expect.element(screen.getByText('file list')).not.toBeInTheDocument()
  })

  it('shows the folder and its files once saved', async () => {
    vi.mocked(fetchEnvFolder).mockResolvedValueOnce('/host/env')
    const screen = await render(withQueryClient(<EnvFiles />))

    // The folder is set in Settings; here it only says which one is listed.
    await expect.element(screen.getByText('/host/env')).toBeInTheDocument()
    await expect
      .element(screen.getByRole('link', { name: 'Change' }))
      .toHaveAttribute('href', '/settings/env-folder')
    await expect.element(screen.getByText('file list')).toBeInTheDocument()
  })

  it('shows the server error', async () => {
    vi.mocked(fetchEnvFolder).mockRejectedValueOnce(
      new Error('cannot read settings')
    )
    const screen = await render(withQueryClient(<EnvFiles />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read settings')
  })
})
