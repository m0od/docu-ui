import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { page } from 'vitest/browser'
import { SidebarProvider } from '@/components/ui/sidebar'
import { AppSidebar } from './app-sidebar'

vi.mock('@/context/layout-provider', () => ({
  useLayout: () => ({ collapsible: 'icon', variant: 'inset' }),
}))
vi.mock('./app-title', () => ({ AppTitle: () => <p>app title</p> }))
vi.mock('./nav-group', () => ({
  NavGroup: ({ title }: { title: string }) => <p>group {title}</p>,
}))
vi.mock('./nav-user', () => ({ NavUser: () => <p>user menu</p> }))

describe('AppSidebar', () => {
  // The template's team switcher and error-page demos are gone; only real sections remain.
  it('shows the app title, the real menu groups and the user menu', async () => {
    // On a phone the sidebar is a closed drawer; test the desktop layout.
    await page.viewport(1280, 800)
    const screen = await render(
      <SidebarProvider>
        <AppSidebar />
      </SidebarProvider>
    )

    await expect.element(screen.getByText('app title')).toBeInTheDocument()
    const groups = screen.getByText(/^group /)
    expect(groups.elements().map((group) => group.textContent)).toEqual([
      'group General',
      'group Other',
    ])
    await expect.element(screen.getByText('user menu')).toBeInTheDocument()
  })
})
