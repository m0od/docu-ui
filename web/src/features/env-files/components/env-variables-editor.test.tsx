import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { ApiError } from '@/lib/api-client'
import { changeEnvVariables, revealEnvValue } from '../api/env-files-api'
import { EnvVariablesEditor } from './env-variables-editor'

vi.mock('../api/env-files-api', () => ({
  changeEnvVariables: vi.fn(),
  revealEnvValue: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const variables = [
  { key: 'KC_DB', line: 1, overridden: false },
  { key: 'KC_PASSWORD', line: 2, overridden: false },
]

function renderEditor() {
  return render(
    withQueryClient(
      <EnvVariablesEditor
        fileName='keycloak.env'
        version='version-1'
        variables={variables}
      />
    )
  )
}

async function addVariable(screen: RenderResult, key: string, value: string) {
  await userEvent.fill(screen.getByLabelText('New key'), key)
  await userEvent.fill(screen.getByLabelText('New value'), value)
  await userEvent.click(screen.getByRole('button', { name: 'Add' }))
}

describe('EnvVariablesEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows no save bar before any change', async () => {
    const screen = await renderEditor()

    await expect
      .element(screen.getByRole('button', { name: 'Review and save' }))
      .not.toBeInTheDocument()
  })

  // One save for all changes, sent in the order they were made, from the
  // version the user was looking at.
  it('reviews and saves every change at once', async () => {
    vi.mocked(changeEnvVariables).mockResolvedValueOnce()
    const screen = await renderEditor()
    await userEvent.click(
      screen.getByRole('button', { name: 'Remove KC_PASSWORD' })
    )
    await addVariable(screen, 'KC_PORT', '8080')
    await expect
      .element(screen.getByText('2 unsaved changes'))
      .toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )
    const dialog = screen.getByRole('alertdialog')
    await expect.element(dialog).toHaveTextContent('− KC_PASSWORD (removed)')
    await expect.element(dialog).toHaveTextContent('+ KC_PORT (added)')
    // Values stay hidden even in the review.
    await expect.element(dialog).not.toHaveTextContent('8080')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await vi.waitFor(() =>
      expect(changeEnvVariables).toHaveBeenCalledWith(
        'keycloak.env',
        'version-1',
        [
          { key: 'KC_PASSWORD', value: null },
          { key: 'KC_PORT', value: '8080' },
        ]
      )
    )
    await expect
      .element(screen.getByRole('button', { name: 'Review and save' }))
      .not.toBeInTheDocument()
  })

  it('labels a changed variable in the review', async () => {
    vi.mocked(revealEnvValue).mockResolvedValueOnce('postgres')
    const screen = await renderEditor()
    await userEvent.click(screen.getByRole('button', { name: 'Edit KC_DB' }))
    await userEvent.fill(screen.getByLabelText('New value of KC_DB'), 'mariadb')
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await expect
      .element(screen.getByText('1 unsaved change'))
      .toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )

    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('~ KC_DB (changed)')
  })

  it('undoes an added variable and discards everything', async () => {
    const screen = await renderEditor()
    await addVariable(screen, 'A', '1')
    await addVariable(screen, 'B', '2')

    await userEvent.click(screen.getByRole('button', { name: 'Undo A' }))
    await expect
      .element(screen.getByText('1 unsaved change'))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Discard' }))

    await expect.element(screen.getByText('added')).not.toBeInTheDocument()
  })

  // A pending change belongs to the line Compose uses, not to an earlier duplicate.
  it('marks only the active line of a duplicated key as changed', async () => {
    const screen = await render(
      withQueryClient(
        <EnvVariablesEditor
          fileName='keycloak.env'
          version='version-1'
          variables={[
            { key: 'KC_DB', line: 1, overridden: true },
            { key: 'KC_DB', line: 2, overridden: false },
          ]}
        />
      )
    )

    await userEvent.click(screen.getByRole('button', { name: 'Remove KC_DB' }))

    await expect.element(screen.getByText('removed')).toBeInTheDocument()
    await expect
      .element(screen.getByText('overridden below'))
      .toBeInTheDocument()
  })

  it('undoes a change on an existing variable', async () => {
    const screen = await renderEditor()
    await userEvent.click(screen.getByRole('button', { name: 'Remove KC_DB' }))

    await userEvent.click(screen.getByRole('button', { name: 'Undo KC_DB' }))

    await expect.element(screen.getByText('removed')).not.toBeInTheDocument()
  })

  // Someone else saved first: say so, keep the user's changes, offer a reload.
  it('keeps changes and offers a reload on a conflict', async () => {
    vi.mocked(changeEnvVariables).mockRejectedValueOnce(
      new ApiError(409, {
        error: 'the file changed since you opened it: reload and try again',
      })
    )
    const screen = await renderEditor()
    await addVariable(screen, 'A', '1')
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and save' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('the file changed since you opened it')
    await userEvent.click(screen.getByRole('button', { name: 'Reload file' }))
    await expect.element(screen.getByRole('alert')).not.toBeInTheDocument()
    await expect
      .element(screen.getByText('1 unsaved change'))
      .toBeInTheDocument()
  })

  it('shows other save errors without a reload link', async () => {
    vi.mocked(changeEnvVariables).mockRejectedValueOnce(
      new Error('cannot save the env file: permission denied')
    )
    const screen = await renderEditor()
    await addVariable(screen, 'A', '1')
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
})
