import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { TurnOnSignInForm } from './turn-on-sign-in-form'

const navigate = vi.fn()

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }))

async function submit(
  screen: RenderResult,
  username: string,
  password: string,
  confirmPassword: string
) {
  await userEvent.fill(screen.getByLabelText('Username'), username)
  await userEvent.fill(
    screen.getByLabelText('Password', { exact: true }),
    password
  )
  await userEvent.fill(
    screen.getByLabelText('Confirm password'),
    confirmPassword
  )
  await userEvent.click(screen.getByRole('button', { name: 'Turn on sign-in' }))
}

describe('TurnOnSignInForm', () => {
  beforeEach(() => {
    navigate.mockReset()
    useAuthStore
      .getState()
      .auth.setUser({ username: 'anonymous', signIn: false })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // From now on every request needs a session, this browser's too: straight to sign-in.
  it('creates the account and goes to sign-in', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        Response.json({ username: 'tungpt' }, { status: 201 })
      )
    const screen = await render(withQueryClient(<TurnOnSignInForm />))
    await submit(screen, 'tungpt', 'a-long-password', 'a-long-password')

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({ to: '/sign-in', replace: true })
    )
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/account/sign-in',
      expect.objectContaining({
        body: '{"username":"tungpt","password":"a-long-password"}',
      })
    )
    // The route guard must ask the server again, not trust the old "sign-in off" user.
    expect(useAuthStore.getState().auth.user).toBeNull()
  })

  it.each([
    [
      'a bad username',
      'a',
      'a-long-password',
      'a-long-password',
      'Username: use 3-64',
    ],
    [
      'a short password',
      'admin',
      'short',
      'short',
      'Password must be 12-256 characters.',
    ],
    [
      'a too long password',
      'admin',
      'x'.repeat(257),
      'x'.repeat(257),
      'Password must be 12-256 characters.',
    ],
    [
      'a mistyped confirmation',
      'admin',
      'a-long-password',
      'a-long-passwor',
      "Passwords don't match.",
    ],
  ])(
    'rejects %s before calling the server',
    async (_case, username, password, confirmPassword, problem) => {
      const fetchSpy = vi.spyOn(globalThis, 'fetch')
      const screen = await render(withQueryClient(<TurnOnSignInForm />))
      await submit(screen, username, password, confirmPassword)

      await expect.element(screen.getByRole('alert')).toHaveTextContent(problem)
      expect(fetchSpy).not.toHaveBeenCalledWith(
        'api/account/sign-in',
        expect.anything()
      )
    }
  )

  it('shows why the server refused and blocks double submits', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ error: 'sign-in is already on' }, { status: 409 })
    )
    const screen = await render(withQueryClient(<TurnOnSignInForm />))
    await submit(screen, 'admin', 'a-long-password', 'a-long-password')
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('sign-in is already on')
    expect(navigate).not.toHaveBeenCalled()

    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(new Promise(() => {}))
    await userEvent.click(
      screen.getByRole('button', { name: 'Turn on sign-in' })
    )
    await expect
      .element(screen.getByRole('button', { name: 'Turn on sign-in' }))
      .toBeDisabled()
  })
})
