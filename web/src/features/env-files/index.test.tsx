import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { fetchEnvFolder } from './api/env-files-api'
import { EnvFiles } from './index'

vi.mock('./api/env-files-api', () => ({
  fetchEnvFolder: vi.fn(),
  saveEnvFolder: vi.fn(),
}))
vi.mock('./components/doco-cd-form', () => ({
  DocoCDForm: () => <p>doco-cd settings</p>,
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

  // A fresh install: ask for the folder instead of showing an error or an empty list.
  it('asks for the folder when none is saved', async () => {
    vi.mocked(fetchEnvFolder).mockResolvedValueOnce('')
    const screen = await render(withQueryClient(<EnvFiles />))

    await expect
      .element(screen.getByText(/Choose the folder/))
      .toBeInTheDocument()
    await expect.element(screen.getByText('file list')).not.toBeInTheDocument()
  })

  it('shows the folder and its files once saved', async () => {
    vi.mocked(fetchEnvFolder).mockResolvedValueOnce('/host/env')
    const screen = await render(withQueryClient(<EnvFiles />))

    await expect
      .element(screen.getByLabelText('Env folder'))
      .toHaveValue('/host/env')
    await expect.element(screen.getByText('file list')).toBeInTheDocument()
    await expect
      .element(screen.getByText('doco-cd settings'))
      .toBeInTheDocument()
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
