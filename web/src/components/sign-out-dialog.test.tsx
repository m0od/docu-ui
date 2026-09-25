import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { signOut } from '@/lib/session-api'
import { SignOutDialog } from './sign-out-dialog'

const navigate = vi.fn()
const reset = vi.fn()

const MOCK_HREF = '/settings?tab=1'

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: () => ({
    auth: { reset },
  }),
}))

vi.mock('@/lib/session-api', () => ({ signOut: vi.fn() }))

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useNavigate: () => navigate,
    useLocation: () => ({ href: MOCK_HREF }),
  }
})

describe('SignOutDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('ends the server session, forgets the user and goes to sign-in', async () => {
    vi.mocked(signOut).mockResolvedValueOnce()
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )

    await userEvent.click(getByRole('button', { name: /^Sign out$/i }))

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: '/sign-in',
        search: { redirect: MOCK_HREF },
        replace: true,
      })
    )
    expect(signOut).toHaveBeenCalledOnce()
    expect(reset).toHaveBeenCalledOnce()
  })

  // Showing the sign-in page while the server session is still alive would
  // make the user believe a shared computer is safe to walk away from.
  it('stays signed in and says so when the server call fails', async () => {
    vi.mocked(signOut).mockRejectedValueOnce(new Error('offline'))
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )

    await userEvent.click(getByRole('button', { name: /^Sign out$/i }))

    await vi.waitFor(() => expect(toast.error).toHaveBeenCalledWith('offline'))
    expect(reset).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })

  it('uses a generic message for non-Error failures', async () => {
    vi.mocked(signOut).mockRejectedValueOnce('boom')
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )

    await userEvent.click(getByRole('button', { name: /^Sign out$/i }))

    await vi.waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Sign-out failed.')
    )
  })

  it('does nothing when Cancel is clicked', async () => {
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )

    await userEvent.click(getByRole('button', { name: /^Cancel$/i }))

    expect(signOut).not.toHaveBeenCalled()
    expect(reset).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })
})
