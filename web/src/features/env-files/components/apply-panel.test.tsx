import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import {
  applyEnvFile,
  fetchApplyState,
  saveApplyTarget,
} from '../api/apply-api'
import { ApplyPanel } from './apply-panel'

vi.mock('../api/apply-api', () => ({
  applyEnvFile: vi.fn(),
  fetchApplyState: vi.fn(),
  saveApplyTarget: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))

function renderPanel() {
  return render(
    withQueryClient(<ApplyPanel fileName='keycloak.env' version='version-2' />)
  )
}

describe('ApplyPanel', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  // First visit: ask where the file is used; there is nothing to cancel back to.
  it('asks for the project and services, then shows them', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: null,
      appliedVersion: '',
    })
    vi.mocked(saveApplyTarget).mockResolvedValueOnce({
      target: { project: 'textiq-dev', services: ['keycloak', 'iam'] },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    const saveButton = screen.getByRole('button', { name: 'Save' })
    await expect.element(saveButton).toBeDisabled()
    await expect
      .element(screen.getByRole('button', { name: 'Cancel' }))
      .not.toBeInTheDocument()

    await userEvent.fill(
      screen.getByLabelText('Compose project'),
      ' textiq-dev '
    )
    await userEvent.fill(screen.getByLabelText('Services'), 'keycloak, iam ')
    await userEvent.click(saveButton)

    await expect.element(screen.getByText('keycloak, iam')).toBeInTheDocument()
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
    expect(saveApplyTarget).toHaveBeenCalledWith('keycloak.env', {
      project: 'textiq-dev',
      services: ['keycloak', 'iam'],
    })
  })

  it('shows why the target cannot be saved', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: null,
      appliedVersion: '',
    })
    vi.mocked(saveApplyTarget).mockRejectedValueOnce(
      new Error('the project name must be lowercase')
    )
    const screen = await renderPanel()
    await userEvent.fill(screen.getByLabelText('Compose project'), 'TextIQ')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('the project name must be lowercase')
  })

  it('changes the target starting from the saved one, and can cancel', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: { project: 'textiq-dev', services: ['keycloak', 'iam'] },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Change' }))

    await expect
      .element(screen.getByLabelText('Compose project'))
      .toHaveValue('textiq-dev')
    await expect
      .element(screen.getByLabelText('Services'))
      .toHaveValue('keycloak iam')
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
  })

  // The file on screen is newer than what the containers run: offer Apply for exactly that version.
  it('applies a saved change to the whole project', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: { project: 'textiq-dev', services: [] },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockResolvedValueOnce({
      target: { project: 'textiq-dev', services: [] },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await expect
      .element(screen.getByText('(whole project)', { exact: false }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('Doco-CD recreates every service in textiq-dev')
    await userEvent.click(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Apply' })
    )

    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
    expect(applyEnvFile).toHaveBeenCalledWith('keycloak.env', 'version-2')
  })

  it('names the services it recreates, and shows why applying failed', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: { project: 'textiq-dev', services: ['keycloak'] },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockRejectedValueOnce(
      new Error('doco-cd answered 401: invalid api key')
    )
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    const dialog = screen.getByRole('alertdialog')
    await expect
      .element(dialog)
      .toHaveTextContent('Doco-CD recreates keycloak in textiq-dev')
    await userEvent.click(dialog.getByRole('button', { name: 'Apply' }))

    await expect
      .element(dialog.getByRole('alert'))
      .toHaveTextContent('invalid api key')
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .not.toBeInTheDocument()
  })

  it('shows why the target cannot be read', async () => {
    vi.mocked(fetchApplyState).mockRejectedValueOnce(
      new Error('cannot read settings')
    )
    const screen = await renderPanel()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read settings')
  })
})
