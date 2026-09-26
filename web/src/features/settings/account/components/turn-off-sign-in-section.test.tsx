import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { TurnOffSignInSection } from './turn-off-sign-in-section'

vi.mock('sonner', () => ({ toast: { warning: vi.fn() } }))

function renderSection(totpEnabled: boolean) {
  const queryClient = new QueryClient()
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  const rendering = render(
    <QueryClientProvider client={queryClient}>
      <TurnOffSignInSection totpEnabled={totpEnabled} />
    </QueryClientProvider>
  )
  return { rendering, invalidateSpy }
}

describe('TurnOffSignInSection', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // The banner and the hidden Sign out follow the store, so it must switch at once.
  it('turns sign-in off with the password and switches the UI to no sign-in', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    const { rendering, invalidateSpy } = renderSection(false)
    const screen = await rendering
    await expect
      .element(screen.getByLabelText('Code from the app'))
      .not.toBeInTheDocument()
    const turnOff = screen.getByRole('button', { name: 'Turn off sign-in' })
    await expect.element(turnOff).toBeDisabled()
    await userEvent.fill(screen.getByLabelText('Current password'), 'pw')
    await userEvent.click(turnOff)

    await vi.waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['account'] })
    )
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/account/sign-in/disable',
      expect.objectContaining({ body: '{"currentPassword":"pw","code":""}' })
    )
    expect(useAuthStore.getState().auth.user).toEqual({
      username: 'anonymous',
      signIn: false,
    })
  })

  // With TOTP on, the password alone is not enough, as at sign-in.
  it('needs a 6-digit code when TOTP is on', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    const screen = await renderSection(true).rendering
    await userEvent.fill(screen.getByLabelText('Current password'), 'pw')
    const turnOff = screen.getByRole('button', { name: 'Turn off sign-in' })
    await expect.element(turnOff).toBeDisabled()
    await userEvent.fill(screen.getByLabelText('Code from the app'), '123456')
    await userEvent.click(turnOff)

    await vi.waitFor(() =>
      expect(fetchSpy).toHaveBeenCalledWith(
        'api/account/sign-in/disable',
        expect.objectContaining({
          body: '{"currentPassword":"pw","code":"123456"}',
        })
      )
    )
  })

  it('shows why the server refused and keeps sign-in on', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ error: 'current password is wrong' }, { status: 403 })
    )
    const screen = await renderSection(false).rendering
    await userEvent.fill(screen.getByLabelText('Current password'), 'wrong')
    await userEvent.click(
      screen.getByRole('button', { name: 'Turn off sign-in' })
    )

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('current password is wrong')
    expect(useAuthStore.getState().auth.user?.signIn).toBe(true)
  })

  it('disables the button while turning off', async () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(new Promise(() => {}))
    const screen = await renderSection(false).rendering
    await userEvent.fill(screen.getByLabelText('Current password'), 'pw')
    await userEvent.click(
      screen.getByRole('button', { name: 'Turn off sign-in' })
    )

    await expect
      .element(screen.getByRole('button', { name: 'Turn off sign-in' }))
      .toBeDisabled()
  })
})
