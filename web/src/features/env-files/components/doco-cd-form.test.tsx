import { withQueryClient } from '@/test-utils/query-client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { fetchDocoCD, saveDocoCD } from '../api/apply-api'
import { DocoCDForm } from './doco-cd-form'

vi.mock('../api/apply-api', () => ({
  fetchDocoCD: vi.fn(),
  saveDocoCD: vi.fn(),
}))

function renderForm() {
  return render(withQueryClient(<DocoCDForm />))
}

describe('DocoCDForm', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('saves a new URL and key, then clears the key field', async () => {
    vi.mocked(fetchDocoCD).mockResolvedValueOnce({ url: '', hasApiKey: false })
    vi.mocked(saveDocoCD).mockResolvedValueOnce({
      url: 'http://doco-cd:80',
      hasApiKey: true,
    })
    const screen = await renderForm()
    const saveButton = screen.getByRole('button', { name: 'Save' })
    await expect.element(saveButton).toBeDisabled()
    await expect
      .element(screen.getByLabelText('Doco-CD API key'))
      .toHaveAttribute('placeholder', 'API_SECRET of Doco-CD')

    await userEvent.fill(
      screen.getByLabelText('Doco-CD URL'),
      ' http://doco-cd:80 '
    )
    await userEvent.fill(screen.getByLabelText('Doco-CD API key'), 'api-key')
    await userEvent.click(saveButton)

    await expect
      .element(screen.getByText('Doco-CD settings saved.'))
      .toBeInTheDocument()
    expect(saveDocoCD).toHaveBeenCalledWith('http://doco-cd:80', 'api-key')
    // The key is never shown again; the field only says one is saved.
    await expect
      .element(screen.getByLabelText('Doco-CD API key'))
      .toHaveValue('')
    await expect
      .element(screen.getByLabelText('Doco-CD API key'))
      .toHaveAttribute('placeholder', 'Saved; leave empty to keep it')
  })

  // Rotating the key alone, with the URL unchanged, must be possible.
  it('enables Save for a new key alone', async () => {
    vi.mocked(fetchDocoCD).mockResolvedValueOnce({
      url: 'http://doco-cd',
      hasApiKey: true,
    })
    const screen = await renderForm()
    await expect
      .element(screen.getByLabelText('Doco-CD URL'))
      .toHaveValue('http://doco-cd')
    await userEvent.fill(screen.getByLabelText('Doco-CD API key'), 'new-key')

    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeEnabled()
  })

  // No double submit while the server is still answering.
  it('disables Save while saving', async () => {
    vi.mocked(fetchDocoCD).mockResolvedValueOnce({ url: '', hasApiKey: false })
    vi.mocked(saveDocoCD).mockReturnValueOnce(new Promise(() => {}))
    const screen = await renderForm()
    await userEvent.fill(screen.getByLabelText('Doco-CD URL'), 'http://doco-cd')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('button', { name: 'Save' }))
      .toBeDisabled()
  })

  it('shows why saving failed', async () => {
    vi.mocked(fetchDocoCD).mockResolvedValueOnce({ url: '', hasApiKey: false })
    vi.mocked(saveDocoCD).mockRejectedValueOnce(
      new Error('the Doco-CD URL must look like')
    )
    const screen = await renderForm()
    await userEvent.fill(screen.getByLabelText('Doco-CD URL'), 'doco-cd')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('the Doco-CD URL must look like')
  })

  it('shows why the settings cannot be read', async () => {
    vi.mocked(fetchDocoCD).mockRejectedValueOnce(
      new Error('cannot read settings')
    )
    const screen = await renderForm()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read settings')
  })
})
