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
    withQueryClient(<ApplyPanel fileName='api.env' version='version-2' />)
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
        project: 'shop-dev',
        services: ['api', 'worker'],
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

    await userEvent.fill(screen.getByLabelText('Compose project'), ' shop-dev ')
    await userEvent.fill(screen.getByLabelText('Services'), 'api, worker ')
    await userEvent.click(saveButton)

    await expect
      .element(screen.getByText('shop-dev: api, worker'))
      .toBeInTheDocument()
    await expect.element(screen.getByText('via Doco-CD')).toBeInTheDocument()
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
    expect(saveApplyTarget).toHaveBeenCalledWith('api.env', {
      adapter: 'doco-cd',
      project: 'shop-dev',
      services: ['api', 'worker'],
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
    await userEvent.fill(screen.getByLabelText('Compose project'), 'Shop')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('the project name must be lowercase')
  })

  it('changes the target starting from the saved one, and can cancel', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: {
        adapter: 'doco-cd',
        project: 'shop-dev',
        services: ['api', 'worker'],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Change' }))

    await expect
      .element(screen.getByLabelText('Compose project'))
      .toHaveValue('shop-dev')
    await expect
      .element(screen.getByLabelText('Services'))
      .toHaveValue('api worker')
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
        project: 'shop-dev',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockResolvedValueOnce({
      target: {
        adapter: 'doco-cd',
        project: 'shop-dev',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-2',
    })
    const screen = await renderPanel()
    await expect
      .element(screen.getByText('shop-dev: whole project'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('Doco-CD recreates every service in shop-dev')
    await userEvent.click(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Apply' })
    )

    await expect
      .element(screen.getByText('Applied', { exact: true }))
      .toBeInTheDocument()
    expect(applyEnvFile).toHaveBeenCalledWith('api.env', 'version-2')
  })

  it('names the services it recreates, and shows why applying failed', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: {
        adapter: 'doco-cd',
        project: 'shop-dev',
        services: ['api'],
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
      .toHaveTextContent('Doco-CD recreates api in shop-dev')
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

    expect(saveApplyTarget).toHaveBeenCalledWith('api.env', {
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
        project: 'shop-dev',
        services: ['api'],
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
      'shop-dev'
    )
    await userEvent.fill(screen.getByLabelText('Services'), 'api')
    await userEvent.click(saveButton)

    expect(saveApplyTarget).toHaveBeenCalledWith('api.env', {
      adapter: 'webhook',
      project: 'shop-dev',
      services: ['api'],
      webhook: {
        ...NO_WEBHOOK_INPUT,
        url: 'https://ci.example/own',
        secret: 'secret',
      },
    })
    await expect
      .element(screen.getByText('via own webhook'))
      .toBeInTheDocument()
    await expect.element(screen.getByText('shop-dev: api')).toBeInTheDocument()
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

    expect(saveApplyTarget).toHaveBeenCalledWith('api.env', {
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

  // Compose finds the containers by project name, so the project is required.
  it('applies through Docker Compose on this host', async () => {
    const composeTarget = {
      adapter: 'compose' as const,
      project: 'shop-dev',
      services: ['api'],
      webhook: NO_WEBHOOK,
    }
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: null,
      appliedVersion: '',
    })
    vi.mocked(saveApplyTarget).mockResolvedValueOnce({
      target: composeTarget,
      appliedVersion: 'version-1',
    })
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('radio', { name: /Docker Compose/ }))
    const saveButton = screen.getByRole('button', { name: 'Save' })
    await expect.element(saveButton).toBeDisabled()
    await userEvent.fill(screen.getByLabelText('Compose project'), 'shop-dev')
    await userEvent.fill(screen.getByLabelText('Services'), 'api')
    await userEvent.click(saveButton)

    expect(saveApplyTarget).toHaveBeenCalledWith('api.env', {
      ...composeTarget,
      webhook: NO_WEBHOOK_INPUT,
    })
    await expect
      .element(screen.getByText('via Docker Compose'))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await expect
      .element(screen.getByRole('alertdialog'))
      .toHaveTextContent('Docker Compose recreates api in shop-dev')
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

  // A double click on Apply must recreate the containers once, not restart them twice.
  it('applies once on a double click', async () => {
    vi.mocked(fetchApplyState).mockResolvedValueOnce({
      target: {
        adapter: 'doco-cd',
        project: 'shop-dev',
        services: [],
        webhook: NO_WEBHOOK,
      },
      appliedVersion: 'version-1',
    })
    vi.mocked(applyEnvFile).mockReturnValue(new Promise(() => {}))
    const screen = await renderPanel()
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await userEvent.dblClick(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Apply' })
    )

    await vi.waitFor(() => expect(applyEnvFile).toHaveBeenCalled())
    expect(applyEnvFile).toHaveBeenCalledOnce()
  })
})
