import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { useAuthStore } from '@/stores/auth-store'
import { SignInOffBanner } from './sign-in-off-banner'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, ...linkProps }: React.ComponentProps<'a'> & { to: string }) => (
    <a href={to} {...linkProps} />
  ),
}))

describe('SignInOffBanner', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
  })

  // Anyone on the network can read the secrets; the warning must not be easy to miss.
  it('warns and links to turning sign-in on while sign-in is off', async () => {
    useAuthStore
      .getState()
      .auth.setUser({ username: 'anonymous', signIn: false })
    const screen = await render(<SignInOffBanner />)

    await expect
      .element(screen.getByText(/anyone who reaches this URL/))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('link', { name: 'Turn on sign-in' }))
      .toHaveAttribute('href', '/settings/account')
  })

  it.each([
    ['signed in', { username: 'admin', signIn: true }],
    ['not known yet', null],
  ])('stays hidden when %s', async (_state, user) => {
    useAuthStore.getState().auth.setUser(user)
    const screen = await render(<SignInOffBanner />)

    expect(screen.container.textContent).toBe('')
  })
})
