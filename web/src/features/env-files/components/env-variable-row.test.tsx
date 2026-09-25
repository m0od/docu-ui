import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { Table, TableBody } from '@/components/ui/table'
import { type EnvVariable, revealEnvValue } from '../api/env-files-api'
import { EnvVariableRow } from './env-variable-row'

vi.mock('../api/env-files-api', () => ({ revealEnvValue: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

const MASK = '••••••••'

function renderRow(variable: EnvVariable) {
  return render(
    <Table>
      <TableBody>
        <EnvVariableRow fileName='keycloak.env' variable={variable} />
      </TableBody>
    </Table>
  )
}

const password = { key: 'KC_DB_PASSWORD', line: 4, overridden: false }

describe('EnvVariableRow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Nothing is fetched until asked: opening the page must not load secrets.
  it('starts masked without asking the server', async () => {
    const screen = await renderRow(password)

    await expect.element(screen.getByText(MASK)).toBeInTheDocument()
    expect(revealEnvValue).not.toHaveBeenCalled()
  })

  it('reveals on click and forgets the value on the second click', async () => {
    vi.mocked(revealEnvValue).mockResolvedValueOnce('s3cret')
    const screen = await renderRow(password)

    await userEvent.click(
      screen.getByRole('button', { name: 'Show KC_DB_PASSWORD' })
    )
    await expect.element(screen.getByText('s3cret')).toBeInTheDocument()
    expect(revealEnvValue).toHaveBeenCalledWith(
      'keycloak.env',
      'KC_DB_PASSWORD'
    )

    await userEvent.click(
      screen.getByRole('button', { name: 'Hide KC_DB_PASSWORD' })
    )
    await expect.element(screen.getByText('s3cret')).not.toBeInTheDocument()
    await expect.element(screen.getByText(MASK)).toBeInTheDocument()
  })

  it('stays masked and tells the user when loading fails', async () => {
    vi.mocked(revealEnvValue).mockRejectedValueOnce(
      new Error('session expired: sign in again')
    )
    const screen = await renderRow(password)

    await userEvent.click(screen.getByRole('button', { name: /Show/ }))

    await vi.waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('session expired: sign in again')
    )
    await expect.element(screen.getByText(MASK)).toBeInTheDocument()
  })

  it('uses a generic message for non-Error failures', async () => {
    vi.mocked(revealEnvValue).mockRejectedValueOnce('boom')
    const screen = await renderRow(password)

    await userEvent.click(screen.getByRole('button', { name: /Show/ }))

    await vi.waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Cannot load value.')
    )
  })

  // The server returns the value Compose uses (the last line), so a
  // reveal button on an earlier duplicate would show the wrong value.
  it('marks an overridden line and offers no reveal', async () => {
    const screen = await renderRow({ key: 'KC_DB', line: 2, overridden: true })

    await expect
      .element(screen.getByText('overridden below'))
      .toBeInTheDocument()
    await expect.element(screen.getByRole('button')).not.toBeInTheDocument()
  })
})
