import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { ProfileDropdown } from './profile-dropdown'

vi.mock('@tanstack/react-router', () => ({
  // Forwards the menu item props (role, handlers) that asChild passes down.
  Link: ({ to, ...linkProps }: React.ComponentProps<'a'> & { to: string }) => (
    <a href={to} {...linkProps} />
  ),
}))
vi.mock('@/components/sign-out-dialog', () => ({
  SignOutDialog: ({ open }: { open: boolean }) =>
    open ? <p>sign out dialog</p> : null,
}))

describe('ProfileDropdown', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })
  })

  // No template Billing or New Team, and no shortcut hints that no key handler backs.
  it('offers account, settings and sign out only', async () => {
    const screen = await render(<ProfileDropdown />)
    await userEvent.click(screen.getByRole('button'))

    const menuItems = screen.getByRole('menuitem')
    expect(menuItems.elements().map((item) => item.textContent)).toEqual([
      'Account',
      'Settings',
      'Sign out',
    ])

    await userEvent.click(menuItems.last())
    await expect
      .element(screen.getByText('sign out dialog'))
      .toBeInTheDocument()
  })

  it('offers no sign out while sign-in is off', async () => {
    useAuthStore
      .getState()
      .auth.setUser({ username: 'anonymous', signIn: false })
    const screen = await render(<ProfileDropdown />)
    await userEvent.click(screen.getByRole('button'))

    const menuItems = screen.getByRole('menuitem')
    await expect.element(menuItems.first()).toHaveTextContent('Account')
    expect(menuItems.elements().map((item) => item.textContent)).toEqual([
      'Account',
      'Settings',
    ])
  })

  // Before /api/auth/me answers there is no user yet; the menu must still render.
  it('renders before the user is known', async () => {
    useAuthStore.getState().auth.setUser(null)
    const screen = await render(<ProfileDropdown />)
    await expect.element(screen.getByRole('button')).toBeInTheDocument()
  })
})
