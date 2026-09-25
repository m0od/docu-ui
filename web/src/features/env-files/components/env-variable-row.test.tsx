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
const onChange = vi.fn()
const onUndo = vi.fn()
const password = { key: 'KC_DB_PASSWORD', line: 4, overridden: false }

function renderRow(
  variable: EnvVariable,
  pendingValue: string | null | undefined = undefined
) {
  return render(
    <Table>
      <TableBody>
        <EnvVariableRow
          fileName='keycloak.env'
          variable={variable}
          pendingValue={pendingValue}
          onChange={onChange}
          onUndo={onUndo}
        />
      </TableBody>
    </Table>
  )
}

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

  // Editing starts from the current value, so a small fix is not a full retype.
  it('edits starting from the value in the file', async () => {
    vi.mocked(revealEnvValue).mockResolvedValueOnce('old')
    const screen = await renderRow(password)

    await userEvent.click(screen.getByRole('button', { name: /Edit/ }))
    const valueInput = screen.getByLabelText('New value of KC_DB_PASSWORD')
    await expect.element(valueInput).toHaveValue('old')
    await userEvent.fill(valueInput, 'new')
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(onChange).toHaveBeenCalledWith('new')
    await expect.element(valueInput).not.toBeInTheDocument()
  })

  it('reuses a revealed value instead of loading it again', async () => {
    vi.mocked(revealEnvValue).mockResolvedValueOnce('shown')
    const screen = await renderRow(password)
    await userEvent.click(screen.getByRole('button', { name: /Show/ }))
    await expect.element(screen.getByText('shown')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /Edit/ }))

    await expect
      .element(screen.getByLabelText(/New value/))
      .toHaveValue('shown')
    expect(revealEnvValue).toHaveBeenCalledOnce()
  })

  it('does not open the editor when the value cannot be loaded', async () => {
    vi.mocked(revealEnvValue).mockRejectedValueOnce(new Error('gone'))
    const screen = await renderRow(password)

    await userEvent.click(screen.getByRole('button', { name: /Edit/ }))

    await vi.waitFor(() => expect(toast.error).toHaveBeenCalled())
    await expect
      .element(screen.getByLabelText(/New value/))
      .not.toBeInTheDocument()
  })

  it('cancels an edit without a change', async () => {
    const screen = await renderRow(password, 'unsaved')

    await userEvent.click(screen.getByRole('button', { name: /Edit/ }))
    await expect
      .element(screen.getByLabelText(/New value/))
      .toHaveValue('unsaved')
    await userEvent.click(screen.getByRole('button', { name: 'Cancel edit' }))

    expect(onChange).not.toHaveBeenCalled()
    expect(revealEnvValue).not.toHaveBeenCalled()
  })

  it('removes on click', async () => {
    const screen = await renderRow(password)

    await userEvent.click(screen.getByRole('button', { name: /Remove/ }))

    expect(onChange).toHaveBeenCalledWith(null)
  })

  it('marks a removed variable and offers only undo', async () => {
    const screen = await renderRow(password, null)

    await expect.element(screen.getByText('removed')).toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /Remove/ }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /Edit/ }))
      .not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Undo/ }))
    expect(onUndo).toHaveBeenCalledOnce()
  })

  // An unsaved new value is a secret too; it is not echoed back on the page.
  it('keeps a changed value masked', async () => {
    const screen = await renderRow(password, 'typed-secret')

    await expect.element(screen.getByText('changed')).toBeInTheDocument()
    await expect
      .element(screen.getByText('typed-secret'))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /Show/ }))
      .not.toBeInTheDocument()
  })

  // The server acts on the line Compose uses; actions on an earlier
  // duplicate would show or change the wrong line.
  it('offers no actions on an overridden line', async () => {
    const screen = await renderRow({ key: 'KC_DB', line: 2, overridden: true })

    await expect
      .element(screen.getByText('overridden below'))
      .toBeInTheDocument()
    await expect.element(screen.getByRole('button')).not.toBeInTheDocument()
  })
})
