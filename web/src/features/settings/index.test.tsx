import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { Settings } from './index'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  Outlet: () => <p>section</p>,
}))
vi.mock('./components/sidebar-nav', () => ({
  SidebarNav: ({ items }: { items: { title: string; href: string }[] }) => (
    <ul>
      {items.map((item) => (
        <li key={item.href}>{`${item.title} ${item.href}`}</li>
      ))}
    </ul>
  ),
}))
// The page header needs the app's layout providers; it is not what this test is about.
vi.mock('@/components/layout/header', () => ({ Header: () => null }))
vi.mock('@/components/layout/main', () => ({
  Main: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}))

describe('Settings', () => {
  // Only real sections: the template's Profile, Account, Notifications and Display are gone.
  it('lists the env folder, apply and appearance sections', async () => {
    const screen = await render(<Settings />)

    const sections = screen.getByRole('listitem')
    expect(sections.elements().map((item) => item.textContent)).toEqual([
      'Env folder /settings/env-folder',
      'Apply /settings/apply',
      'Appearance /settings/appearance',
    ])
    await expect.element(screen.getByText('section')).toBeInTheDocument()
  })
})
