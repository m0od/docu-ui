import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { page, userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { SidebarProvider } from '@/components/ui/sidebar'
import { NavUser } from './nav-user'

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

describe('NavUser', () => {
  beforeEach(async () => {
    // Desktop by default; below 768px the sidebar switches to its phone layout.
    await page.viewport(1280, 800)
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })
  })

  function renderNavUser() {
    return render(
      <SidebarProvider>
        <NavUser />
      </SidebarProvider>
    )
  }

  // Only entries that lead somewhere real: no template Billing, Upgrade or Notifications.
  it('offers account, settings and sign out only', async () => {
    const screen = await renderNavUser()
    await userEvent.click(screen.getByRole('button', { name: /admin/ }))

    await expect
      .element(screen.getByRole('menu'))
      .toHaveAttribute('data-side', 'right')
    const menuItems = screen.getByRole('menuitem')
    expect(menuItems.elements().map((item) => item.textContent)).toEqual([
      'Account',
      'Settings',
      'Sign out',
    ])
    await expect
      .element(menuItems.first())
      .toHaveAttribute('href', '/settings/account')
    await expect.element(menuItems.nth(1)).toHaveAttribute('href', '/settings')

    await userEvent.click(menuItems.last())
    await expect
      .element(screen.getByText('sign out dialog'))
      .toBeInTheDocument()
  })

  it('offers no sign out while sign-in is off', async () => {
    useAuthStore
      .getState()
      .auth.setUser({ username: 'anonymous', signIn: false })
    const screen = await renderNavUser()
    await userEvent.click(screen.getByRole('button', { name: /anonymous/ }))

    const menuItems = screen.getByRole('menuitem')
    await expect.element(menuItems.first()).toHaveTextContent('Account')
    expect(menuItems.elements().map((item) => item.textContent)).toEqual([
      'Account',
      'Settings',
    ])
  })

  // On a phone the sidebar fills the screen; a menu to its right would open off-screen.
  it('opens the menu below on a phone', async () => {
    await page.viewport(375, 800)
    const screen = await renderNavUser()
    await userEvent.click(screen.getByRole('button', { name: /admin/ }))

    await expect
      .element(screen.getByRole('menu'))
      .toHaveAttribute('data-side', 'bottom')
  })

  // Before /api/auth/me answers there is no user yet; the menu must still render.
  it('renders before the user is known', async () => {
    useAuthStore.getState().auth.setUser(null)
    const screen = await render(
      <SidebarProvider>
        <NavUser />
      </SidebarProvider>
    )
    await expect.element(screen.getByRole('button')).toBeInTheDocument()
  })
})
