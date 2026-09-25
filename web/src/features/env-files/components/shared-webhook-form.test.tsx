import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { fetchSharedWebhook, saveSharedWebhook } from '../api/apply-api'
import { SharedWebhookForm } from './shared-webhook-form'

vi.mock('../api/apply-api', () => ({
  fetchSharedWebhook: vi.fn(),
  saveSharedWebhook: vi.fn(),
}))

const NOTHING_SAVED = {
  url: '',
  hasSecret: false,
  headerName: '',
  hasHeaderValue: false,
}

function renderForm() {
  return render(withQueryClient(<SharedWebhookForm />))
}

describe('SharedWebhookForm', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('saves the webhook, then clears the secret fields', async () => {
    vi.mocked(fetchSharedWebhook).mockResolvedValueOnce(NOTHING_SAVED)
    vi.mocked(saveSharedWebhook).mockResolvedValueOnce({
      url: 'https://ci.example/hook',
      hasSecret: true,
      headerName: 'Authorization',
      hasHeaderValue: true,
    })
    const screen = await renderForm()
    const saveButton = screen.getByRole('button', { name: 'Save' })
    // Without a URL there is nowhere to post.
    await expect.element(saveButton).toBeDisabled()

    await userEvent.fill(
      screen.getByLabelText('Webhook URL'),
      ' https://ci.example/hook '
    )
    await userEvent.fill(screen.getByLabelText('Signing secret'), 'secret')
    await userEvent.fill(screen.getByLabelText('Header name'), 'Authorization')
    await userEvent.fill(screen.getByLabelText('Header value'), 'Bearer token')
    await userEvent.click(saveButton)

    await expect.element(screen.getByText('Webhook saved.')).toBeInTheDocument()
    expect(saveSharedWebhook).toHaveBeenCalledWith({
      url: 'https://ci.example/hook',
      secret: 'secret',
      headerName: 'Authorization',
      headerValue: 'Bearer token',
    })
    // Secrets are never shown again; the fields only say one is saved.
    for (const label of ['Signing secret', 'Header value']) {
      await expect.element(screen.getByLabelText(label)).toHaveValue('')
      await expect
        .element(screen.getByLabelText(label))
        .toHaveAttribute('placeholder', 'Saved; leave empty to keep it')
    }
  })

  it('shows why the webhook cannot be saved', async () => {
    vi.mocked(fetchSharedWebhook).mockResolvedValueOnce({
      url: 'https://ci.example/hook',
      hasSecret: false,
      headerName: '',
      hasHeaderValue: false,
    })
    vi.mocked(saveSharedWebhook).mockRejectedValueOnce(
      new Error('the header name is not valid')
    )
    const screen = await renderForm()
    await expect
      .element(screen.getByLabelText('Signing secret'))
      .toHaveAttribute(
        'placeholder',
        'Optional: signs the body with HMAC-SHA256'
      )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('the header name is not valid')
  })

  it('disables Save while saving', async () => {
    vi.mocked(fetchSharedWebhook).mockResolvedValueOnce({
      ...NOTHING_SAVED,
      url: 'https://ci.example/hook',
    })
    vi.mocked(saveSharedWebhook).mockReturnValueOnce(new Promise(() => {}))
    const screen = await renderForm()
    const saveButton = screen.getByRole('button', { name: 'Save' })
    await userEvent.click(saveButton)

    await expect.element(saveButton).toBeDisabled()
  })

  it('shows why the webhook cannot be read', async () => {
    vi.mocked(fetchSharedWebhook).mockRejectedValueOnce(
      new Error('cannot read settings')
    )
    const screen = await renderForm()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read settings')
  })
})
