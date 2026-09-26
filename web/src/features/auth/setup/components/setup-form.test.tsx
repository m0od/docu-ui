import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import {
  createFirstAccount,
  fetchTotpSecret,
  skipSignIn,
} from '@/features/auth/setup/api/setup-api'
import { SetupForm } from './setup-form'

const navigate = vi.fn()

vi.mock('@tanstack/react-router', async (original) => {
  const actual = await original<typeof import('@tanstack/react-router')>()
  return { ...actual, useNavigate: () => navigate }
})

vi.mock('@/features/auth/setup/api/setup-api', async (original) => {
  const actual =
    await original<typeof import('@/features/auth/setup/api/setup-api')>()
  return {
    ...actual,
    createFirstAccount: vi.fn(),
    fetchTotpSecret: vi.fn(),
    skipSignIn: vi.fn(),
  }
})

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}))

const TOTP_SECRET = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP'

async function fillAccount(screen: RenderResult, confirmPassword: string) {
  await userEvent.fill(screen.getByLabelText('Setup token'), ' token-from-log ')
  await userEvent.fill(screen.getByLabelText('Username'), 'tungpt')
  await userEvent.fill(
    screen.getByLabelText('Password', { exact: true }),
    'a-long-password'
  )
  await userEvent.fill(
    screen.getByLabelText('Confirm password'),
    confirmPassword
  )
}

describe('SetupForm', () => {
  let screen: RenderResult

  beforeEach(async () => {
    vi.clearAllMocks()
    screen = await render(<SetupForm />)
  })

  // TOTP is optional: an admin who skips it must still get an account, with no secret stored.
  it('creates the account without TOTP and goes to sign-in', async () => {
    await fillAccount(screen, 'a-long-password')
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({ to: '/sign-in', replace: true })
    )
    expect(createFirstAccount).toHaveBeenCalledWith({
      setupToken: 'token-from-log',
      username: 'tungpt',
      password: 'a-long-password',
      totpSecret: '',
      totpCode: '',
    })
  })

  it('does not submit when the passwords differ', async () => {
    await fillAccount(screen, 'another-password')
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )

    await expect
      .element(screen.getByText("Passwords don't match."))
      .toBeVisible()
    expect(createFirstAccount).not.toHaveBeenCalled()
  })

  // The server's reason (e.g. wrong setup token) must reach the admin, or they cannot fix it.
  it('shows the server error and stays on the page', async () => {
    vi.mocked(createFirstAccount).mockRejectedValueOnce(
      new Error('setup token is wrong: copy it from the server log')
    )
    await fillAccount(screen, 'a-long-password')
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('setup token is wrong')
    expect(navigate).not.toHaveBeenCalled()
  })

  // Saving a secret the admin never scanned would lock them out, so a code is required when TOTP is on.
  it('requires a code from the app when TOTP is enabled', async () => {
    vi.mocked(fetchTotpSecret).mockResolvedValueOnce(TOTP_SECRET)
    await fillAccount(screen, 'a-long-password')
    await userEvent.click(screen.getByRole('switch'))
    await expect.element(screen.getByText(TOTP_SECRET)).toBeVisible()

    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )
    await expect
      .element(
        screen.getByText('Enter the 6-digit code from your authenticator app.')
      )
      .toBeVisible()
    expect(createFirstAccount).not.toHaveBeenCalled()

    await userEvent.fill(screen.getByLabelText('Code from the app'), '123456')
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )
    await vi.waitFor(() =>
      expect(createFirstAccount).toHaveBeenCalledWith(
        expect.objectContaining({ totpSecret: TOTP_SECRET, totpCode: '123456' })
      )
    )
  })

  // Turning TOTP off and on again must keep the QR code already scanned.
  it('keeps the same secret when TOTP is switched off and on', async () => {
    vi.mocked(fetchTotpSecret).mockResolvedValueOnce(TOTP_SECRET)
    await userEvent.click(screen.getByRole('switch'))
    await expect.element(screen.getByText(TOTP_SECRET)).toBeVisible()
    await userEvent.click(screen.getByRole('switch'))
    await userEvent.click(screen.getByRole('switch'))

    await expect.element(screen.getByText(TOTP_SECRET)).toBeVisible()
    expect(fetchTotpSecret).toHaveBeenCalledTimes(1)
  })

  it('turns the switch back off when the secret cannot be fetched', async () => {
    vi.mocked(fetchTotpSecret).mockRejectedValueOnce(
      new Error('setup already completed')
    )
    await userEvent.click(screen.getByRole('switch'))

    await expect.element(screen.getByRole('switch')).not.toBeChecked()
  })

  // Skipping sign-in still needs the token, but no account fields: those would only be ignored.
  it('finishes setup without sign-in after a clear warning', async () => {
    vi.mocked(skipSignIn).mockResolvedValueOnce()
    await userEvent.click(screen.getByRole('checkbox'))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('anyone who reaches this URL can read')
    await expect
      .element(screen.getByLabelText('Username'))
      .not.toBeInTheDocument()
    await userEvent.fill(
      screen.getByLabelText('Setup token'),
      ' token-from-log '
    )
    await userEvent.click(
      screen.getByRole('button', { name: /Finish setup without sign-in/ })
    )

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({ to: '/', replace: true })
    )
    expect(skipSignIn).toHaveBeenCalledWith('token-from-log')
    expect(createFirstAccount).not.toHaveBeenCalled()
  })

  it('shows why skipping sign-in failed', async () => {
    vi.mocked(skipSignIn).mockRejectedValueOnce(
      new Error('setup token is wrong: copy it from the server log')
    )
    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.fill(screen.getByLabelText('Setup token'), 'wrong')
    await userEvent.click(
      screen.getByRole('button', { name: /Finish setup without sign-in/ })
    )

    await expect
      .element(
        screen.getByText('setup token is wrong: copy it from the server log')
      )
      .toBeInTheDocument()
    expect(navigate).not.toHaveBeenCalled()
  })

  // Unticking brings the account fields back, with their rules.
  it('asks for the account again after unticking', async () => {
    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.fill(screen.getByLabelText('Setup token'), 'token')
    await userEvent.fill(screen.getByLabelText('Username'), 'a')
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )

    await expect
      .element(
        screen.getByText('Use 3-64 letters, digits, dot, dash or underscore.')
      )
      .toBeInTheDocument()
    await expect
      .element(
        screen.getByText('Password must be at least 12 characters long.')
      )
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Please confirm your password.'))
      .toBeInTheDocument()
  })

  it('rejects a password over 256 characters', async () => {
    await userEvent.fill(
      screen.getByLabelText('Password', { exact: true }),
      'x'.repeat(257)
    )
    await userEvent.click(
      screen.getByRole('button', { name: /Create admin account/ })
    )

    await expect
      .element(
        screen.getByText('Password must be at most 256 characters long.')
      )
      .toBeInTheDocument()
  })
})
