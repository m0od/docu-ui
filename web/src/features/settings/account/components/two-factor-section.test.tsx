import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import {
  disableTotp,
  enableTotp,
  fetchAccountTotpSecret,
} from '../api/account-api'
import { TwoFactorSection } from './two-factor-section'

vi.mock('../api/account-api', () => ({
  disableTotp: vi.fn(),
  enableTotp: vi.fn(),
  fetchAccountTotpSecret: vi.fn(),
}))

const TOTP_SECRET = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP'

function renderSection(totpEnabled: boolean) {
  const queryClient = new QueryClient()
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  const screen = render(
    <QueryClientProvider client={queryClient}>
      <TwoFactorSection username='admin' totpEnabled={totpEnabled} />
    </QueryClientProvider>
  )
  return { screen, invalidateSpy }
}

async function confirmWith(screen: RenderResult, buttonName: string) {
  await userEvent.fill(screen.getByLabelText('Current password'), 'pw')
  await userEvent.fill(screen.getByLabelText('Code from the app'), '123456')
  await userEvent.click(screen.getByRole('button', { name: buttonName }))
}

describe('TwoFactorSection', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  // The secret is only saved once a code from it works, so a QR never scanned cannot lock the admin out.
  it('turns TOTP on with a code from the new secret', async () => {
    vi.mocked(fetchAccountTotpSecret).mockResolvedValueOnce(TOTP_SECRET)
    vi.mocked(enableTotp).mockResolvedValueOnce()
    const { screen: rendering, invalidateSpy } = renderSection(false)
    const screen = await rendering
    await expect.element(screen.getByText('Off')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Set up' }))

    await expect.element(screen.getByText(TOTP_SECRET)).toBeVisible()
    expect(screen.container.querySelector('svg title')?.textContent).toBe(
      'TOTP QR code'
    )
    const turnOn = screen.getByRole('button', { name: 'Turn on' })
    await expect.element(turnOn).toBeDisabled()
    await confirmWith(screen, 'Turn on')

    await vi.waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['account'] })
    )
    expect(enableTotp).toHaveBeenCalledWith({
      currentPassword: 'pw',
      secret: TOTP_SECRET,
      code: '123456',
    })
    await expect.element(screen.getByText(TOTP_SECRET)).not.toBeInTheDocument()
  })

  it('cancels the setup without saving', async () => {
    vi.mocked(fetchAccountTotpSecret).mockResolvedValueOnce(TOTP_SECRET)
    const screen = await renderSection(false).screen
    await userEvent.click(screen.getByRole('button', { name: 'Set up' }))
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await expect
      .element(screen.getByRole('button', { name: 'Set up' }))
      .toBeInTheDocument()
    expect(enableTotp).not.toHaveBeenCalled()
  })

  it('shows why no secret could be made', async () => {
    vi.mocked(fetchAccountTotpSecret).mockRejectedValueOnce(
      new Error('server down')
    )
    const screen = await renderSection(false).screen
    await userEvent.click(screen.getByRole('button', { name: 'Set up' }))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('server down')
  })

  it('disables Set up while the secret loads', async () => {
    vi.mocked(fetchAccountTotpSecret).mockReturnValueOnce(new Promise(() => {}))
    const screen = await renderSection(false).screen
    await userEvent.click(screen.getByRole('button', { name: 'Set up' }))

    await expect
      .element(screen.getByRole('button', { name: 'Set up' }))
      .toBeDisabled()
  })

  // Turning off needs both factors too: an open session alone must not weaken sign-in.
  it('turns TOTP off with the password and a code', async () => {
    vi.mocked(disableTotp).mockResolvedValueOnce()
    const { screen: rendering, invalidateSpy } = renderSection(true)
    const screen = await rendering
    await expect
      .element(screen.getByText('On', { exact: true }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Cancel' }))
      .not.toBeInTheDocument()
    await confirmWith(screen, 'Turn off')

    await vi.waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['account'] })
    )
    expect(disableTotp).toHaveBeenCalledWith({
      currentPassword: 'pw',
      code: '123456',
    })
  })

  it('shows why the server refused and blocks double submits', async () => {
    vi.mocked(disableTotp).mockRejectedValueOnce(
      new Error('TOTP code is wrong')
    )
    const screen = await renderSection(true).screen
    await confirmWith(screen, 'Turn off')
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('TOTP code is wrong')

    vi.mocked(disableTotp).mockReturnValueOnce(new Promise(() => {}))
    await userEvent.click(screen.getByRole('button', { name: 'Turn off' }))
    await expect
      .element(screen.getByRole('button', { name: 'Turn off' }))
      .toBeDisabled()
  })
})
