import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { listEnvFiles } from '../api/env-files-api'
import { EnvFileList } from './env-file-list'

vi.mock('../api/env-files-api', () => ({ listEnvFiles: vi.fn() }))

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    params,
    className,
  }: {
    children: React.ReactNode
    params: { name: string }
    className?: string
  }) => (
    <a href={`/env-files/${params.name}`} className={className}>
      {children}
    </a>
  ),
}))

describe('EnvFileList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('links every file to its page', async () => {
    vi.mocked(listEnvFiles).mockResolvedValueOnce(['airflow.env', 'kc.env'])
    const screen = await render(withQueryClient(<EnvFileList />))

    await expect
      .element(screen.getByRole('link', { name: 'airflow.env' }))
      .toHaveAttribute('href', '/env-files/airflow.env')
    await expect
      .element(screen.getByRole('link', { name: 'kc.env' }))
      .toBeInTheDocument()
  })

  it('says so when the folder has no env files', async () => {
    vi.mocked(listEnvFiles).mockResolvedValueOnce([])
    const screen = await render(withQueryClient(<EnvFileList />))

    await expect
      .element(screen.getByText(/files in this folder/))
      .toBeInTheDocument()
  })

  // E.g. the volume was unmounted after the folder was saved.
  it('shows the server error', async () => {
    vi.mocked(listEnvFiles).mockRejectedValueOnce(
      new Error('cannot read the env folder: no such file or directory')
    )
    const screen = await render(withQueryClient(<EnvFileList />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read the env folder')
  })
})
