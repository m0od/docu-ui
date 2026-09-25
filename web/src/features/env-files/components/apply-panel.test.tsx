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

const NO_WEBHOOK = {
  url: '',
  hasSecret: false,
  headerName: '',
  hasHeaderValue: false,
}
const NO_WEBHOOK_INPUT = {
  url: '',
  secret: '',
  headerName: '',
  headerValue: '',
}

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
      target: {
        adapter: 'doco-cd',
        project: 'textiq-dev',
        services: ['keycloak', 'iam'],
        webhook: NO_WEBHOOK,
      },
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

    await expect
      .element(screen.getByText('textiq-dev: keycloak, iam'))
      .toBeInTheDocument()
    await expect.element(screen.getByText('via Doco-CD')).toBeInTheDocument()
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
    expect(saveApplyTarget).toHaveBeenCalledWith('keycloak.env', {
      adapter: 'doco-cd',
      project: 'textiq-dev',
      services: ['keycloak', 'iam'],
      webhook: NO_WEBHOOK_INPUT,
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
      target: {
        adapter: 'doco-cd',
        project: 'textiq-dev',
        services: ['keycloak', 'iam'],
        webhook: NO_WEBHOOK,
      },
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
      target: {
        adapter: 'doco-cd',
        project: 'textiq-dev',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockResolvedValueOnce({
      target: {
        adapter: 'doco-cd',
        project: 'textiq-dev',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await expect
      .element(screen.getByText('textiq-dev: whole project'))
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
      target: {
        adapter: 'doco-cd',
        project: 'textiq-dev',
        services: ['keycloak'],
        webhook: NO_WEBHOOK,
      },
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

  // A webhook receiver may not use Compose at all, so the project is optional.
  it('sends a file to the shared webhook without a project', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: null,
      appliedVersion: '',
    })
    vi.mocked(saveApplyTarget).mockResolvedValueOnce({
      target: {
        adapter: 'webhook',
        project: '',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockResolvedValueOnce({
      target: {
        adapter: 'webhook',
        project: '',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('radio', { name: /Webhook/ }))
    await expect
      .element(screen.getByLabelText('Compose project (optional)'))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(saveApplyTarget).toHaveBeenCalledWith('keycloak.env', {
      adapter: 'webhook',
      project: '',
      services: [],
      webhook: NO_WEBHOOK_INPUT,
    })
    await expect
      .element(screen.getByText('via shared webhook'))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    const dialog = screen.getByRole('alertdialog')
    await expect
      .element(dialog)
      .toHaveTextContent(
        'Docu-UI notifies the shared webhook to apply whole project'
      )
    await userEvent.click(dialog.getByRole('button', { name: 'Apply' }))
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
  })

  it('gives a file its own webhook, which needs a URL', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: null,
      appliedVersion: '',
    })
    vi.mocked(saveApplyTarget).mockResolvedValueOnce({
      target: {
        adapter: 'webhook',
        project: 'textiq-dev',
        services: ['keycloak'],
        webhook: {
          url: 'https://ci.example/own',
          hasSecret: true,
          headerName: '',
          hasHeaderValue: false,
        },
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('radio', { name: /Webhook/ }))
    await userEvent.click(screen.getByRole('checkbox'))
    const saveButton = screen.getByRole('button', { name: 'Save' })
    await expect.element(saveButton).toBeDisabled()

    await userEvent.fill(
      screen.getByLabelText('Webhook URL'),
      ' https://ci.example/own '
    )
    await userEvent.fill(screen.getByLabelText('Signing secret'), 'secret')
    await userEvent.fill(
      screen.getByLabelText('Compose project (optional)'),
      'textiq-dev'
    )
    await userEvent.fill(screen.getByLabelText('Services'), 'keycloak')
    await userEvent.click(saveButton)

    expect(saveApplyTarget).toHaveBeenCalledWith('keycloak.env', {
      adapter: 'webhook',
      project: 'textiq-dev',
      services: ['keycloak'],
      webhook: {
        ...NO_WEBHOOK_INPUT,
        url: 'https://ci.example/own',
        secret: 'secret',
      },
    })
    await expect
      .element(screen.getByText('via own webhook'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('textiq-dev: keycloak'))
      .toBeInTheDocument()
  })

  // Unticking the box drops the file's own webhook, so the shared one is used again.
  it('changes a file back from its own webhook to the shared one', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: {
        adapter: 'webhook',
        project: '',
        services: [],
        webhook: {
          url: 'https://ci.example/own',
          hasSecret: true,
          headerName: 'Authorization',
          hasHeaderValue: true,
        },
      },
      appliedVersion: 'version-2',
    })
    vi.mocked(saveApplyTarget).mockReturnValueOnce(new Promise(() => {}))
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Change' }))

    await expect.element(screen.getByRole('checkbox')).toBeChecked()
    await expect
      .element(screen.getByLabelText('Webhook URL'))
      .toHaveValue('https://ci.example/own')
    await expect
      .element(screen.getByLabelText('Header name'))
      .toHaveValue('Authorization')
    await expect
      .element(screen.getByLabelText('Signing secret'))
      .toHaveAttribute('placeholder', 'Saved; leave empty to keep it')
    await userEvent.click(screen.getByRole('checkbox'))
    await expect
      .element(screen.getByLabelText('Webhook URL'))
      .not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(saveApplyTarget).toHaveBeenCalledWith('keycloak.env', {
      adapter: 'webhook',
      project: '',
      services: [],
      webhook: NO_WEBHOOK_INPUT,
    })
    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeDisabled()
  })

  it('switches a webhook file to Doco-CD, which needs a project', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: {
        adapter: 'webhook',
        project: '',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Change' }))
    await expect.element(screen.getByRole('checkbox')).not.toBeChecked()
    await userEvent.click(screen.getByRole('radio', { name: /Doco-CD/ }))

    await expect.element(screen.getByRole('checkbox')).not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeDisabled()
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
