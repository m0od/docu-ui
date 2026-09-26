import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { SidebarProvider } from '@/components/ui/sidebar'
import { AppTitle } from './app-title'

const setOpenMobile = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    to,
    onClick,
  }: {
    children: React.ReactNode
    to: string
    onClick: () => void
  }) => (
    // A real navigation would unload the test page.
    <a
      href={to}
      onClick={(event) => {
        event.preventDefault()
        onClick()
      }}
    >
      {children}
    </a>
  ),
}))

vi.mock('@/components/ui/sidebar', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/components/ui/sidebar')>()
  return { ...actual, useSidebar: () => ({ setOpenMobile }) }
})

describe('AppTitle', () => {
  // The sidebar names this app, not the template it was built from.
  it('names Docu-UI and leads home, closing the phone sidebar', async () => {
    const screen = await render(
      <SidebarProvider>
        <AppTitle />
      </SidebarProvider>
    )
    const homeLink = screen.getByRole('link', { name: /Docu-UI/ })
    await expect.element(homeLink).toHaveAttribute('href', '/')
    await expect
      .element(screen.getByText('Shadcn', { exact: false }))
      .not.toBeInTheDocument()

    await userEvent.click(homeLink)
    expect(setOpenMobile).toHaveBeenCalledWith(false)
  })
})
