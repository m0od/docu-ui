import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { type Locator, userEvent } from 'vitest/browser'
import { signIn } from '@/lib/session-api'
import { UserAuthForm } from './user-auth-form'

const navigate = vi.fn()
const setUser = vi.fn()

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: () => ({ auth: { setUser } }),
}))

vi.mock('@/lib/session-api', () => ({ signIn: vi.fn() }))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return { ...actual, useNavigate: () => navigate }
})

const TOTP_PROMPT = 'enter the code from your authenticator app'

describe('UserAuthForm', () => {
  let screen: RenderResult
  let usernameInput: Locator
  let passwordInput: Locator
  let signInButton: Locator

  async function renderForm(redirectTo?: string) {
    screen = await render(<UserAuthForm redirectTo={redirectTo} />)
    usernameInput = screen.getByRole('textbox', { name: /^Username$/i })
    passwordInput = screen.getByLabelText(/^Password$/i)
    signInButton = screen.getByRole('button', { name: /^Sign in$/i })
  }

  async function submitCredentials() {
    await userEvent.fill(usernameInput, 'admin')
    await userEvent.fill(passwordInput, 'correct horse battery')
    await userEvent.click(signInButton)
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Docu-UI has no email-based password reset or social login.
  it('offers only username and password', async () => {
    await renderForm()

    await expect.element(usernameInput).toBeInTheDocument()
    await expect.element(passwordInput).toBeInTheDocument()
    await expect
      .element(screen.getByText(/^Forgot password\?$/i))
      .not.toBeInTheDocument()
    await expect.element(screen.getByText(/GitHub/i)).not.toBeInTheDocument()
  })

  it('does not call the server when fields are empty', async () => {
    await renderForm()

    await userEvent.click(signInButton)

    await expect
      .element(screen.getByText('Please enter your username.'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Please enter your password.'))
      .toBeInTheDocument()
    expect(signIn).not.toHaveBeenCalled()
  })

  it('signs in, remembers the user and goes to the dashboard', async () => {
    vi.mocked(signIn).mockResolvedValueOnce({
      status: 'signed-in',
      username: 'admin',
    })
    await renderForm()

    await submitCredentials()

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({ to: '/', replace: true })
    )
    expect(signIn).toHaveBeenCalledWith({
      username: 'admin',
      password: 'correct horse battery',
      totpCode: '',
    })
    expect(setUser).toHaveBeenCalledWith({ username: 'admin', signIn: true })
  })

  it('returns to the page the user was on before the session ran out', async () => {
    vi.mocked(signIn).mockResolvedValueOnce({
      status: 'signed-in',
      username: 'admin',
    })
    await renderForm('/settings?tab=1')

    await submitCredentials()

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: '/settings?tab=1',
        replace: true,
      })
    )
  })

  // A crafted link must not bounce a freshly signed-in admin to another site.
  it.each(['https://evil.example', '//evil.example', 'javascript:alert(1)'])(
    'ignores the off-site redirect %s',
    async (redirectTo) => {
      vi.mocked(signIn).mockResolvedValueOnce({
        status: 'signed-in',
        username: 'admin',
      })
      await renderForm(redirectTo)

      await submitCredentials()

      await vi.waitFor(() =>
        expect(navigate).toHaveBeenCalledWith({ to: '/', replace: true })
      )
    }
  )

  it('shows the server error and stays on the page', async () => {
    vi.mocked(signIn).mockRejectedValueOnce(
      new Error('invalid username or password')
    )
    await renderForm()

    await submitCredentials()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('invalid username or password')
    expect(setUser).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it('falls back to a generic message for non-Error failures', async () => {
    vi.mocked(signIn).mockRejectedValueOnce('network down')
    await renderForm()

    await submitCredentials()

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('Sign-in failed.')
  })

  describe('account with two-factor authentication', () => {
    beforeEach(async () => {
      vi.mocked(signIn).mockResolvedValueOnce({
        status: 'totp-required',
        message: TOTP_PROMPT,
      })
      await renderForm()
      await submitCredentials()
    })

    it('asks for the code instead of failing, and locks the credentials', async () => {
      await expect
        .element(screen.getByRole('alert'))
        .toHaveTextContent(TOTP_PROMPT)
      await expect
        .element(
          screen.getByText('Code from your authenticator app', { exact: true })
        )
        .toBeInTheDocument()
      await expect.element(usernameInput).toHaveAttribute('readonly')
      expect(navigate).not.toHaveBeenCalled()
    })

    it('requires all 6 digits before calling the server again', async () => {
      await userEvent.keyboard('123')
      await userEvent.click(signInButton)

      await expect
        .element(screen.getByText('Enter the 6-digit code.'))
        .toBeInTheDocument()
      expect(signIn).toHaveBeenCalledOnce()
    })

    it('sends the code with the same credentials and signs in', async () => {
      vi.mocked(signIn).mockResolvedValueOnce({
        status: 'signed-in',
        username: 'admin',
      })

      await userEvent.keyboard('123456')
      await userEvent.click(signInButton)

      await vi.waitFor(() =>
        expect(navigate).toHaveBeenCalledWith({ to: '/', replace: true })
      )
      expect(signIn).toHaveBeenLastCalledWith({
        username: 'admin',
        password: 'correct horse battery',
        totpCode: '123456',
      })
    })

    // Codes are single-use; after a rejection the old digits are useless.
    it('clears a rejected code', async () => {
      vi.mocked(signIn).mockRejectedValueOnce(
        new Error('TOTP code was already used: wait for the next code')
      )

      await userEvent.keyboard('123456')
      await userEvent.click(signInButton)

      await expect
        .element(screen.getByRole('alert'))
        .toHaveTextContent('already used')
      await userEvent.click(signInButton)
      await expect
        .element(screen.getByText('Enter the 6-digit code.'))
        .toBeInTheDocument()
    })
  })
})
