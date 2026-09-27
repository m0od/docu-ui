import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router'
import { routeTree } from '@/routeTree.gen'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { useAuthStore } from '@/stores/auth-store'

// The pages themselves are tested elsewhere; here only the guards in front of them matter.
vi.mock('@/features/env-files', () => ({
  EnvFiles: () => <p>env files page</p>,
}))
vi.mock('@/features/env-files/env-file-view', () => ({
  EnvFileView: () => <p>env file page</p>,
}))
vi.mock('@/features/auth/sign-in', () => ({
  SignIn: () => <p>sign-in page</p>,
}))
vi.mock('@/features/auth/setup', () => ({
  Setup: () => <p>setup page</p>,
}))

type ServerState = {
  setupRequired: boolean
  // Answer of GET api/auth/me: a user, or an HTTP status for "no session" / a broken server.
  currentUser: { username: string; signIn: boolean } | 401 | 500
}

let requestedPaths: string[] = []

function serveApi(server: ServerState) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const path = new URL(String(input), document.baseURI).pathname
    requestedPaths.push(path)
    if (path.endsWith('/api/setup')) {
      return Response.json({ required: server.setupRequired })
    }
    if (path.endsWith('/api/auth/me')) {
      if (typeof server.currentUser === 'number') {
        return Response.json({ error: 'no' }, { status: server.currentUser })
      }
      return Response.json(server.currentUser)
    }
    throw new Error(`unexpected request ${path}`)
  })
}

function meRequests() {
  return requestedPaths.filter((path) => path.endsWith('/api/auth/me')).length
}

// The app's real route tree, so the beforeLoad guards run exactly as in the browser.
async function openApp(startPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [startPath] }),
    context: { queryClient },
  })
  const screen = await render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return { router, screen }
}

const signedInAdmin = { username: 'admin', signIn: true }
const signInOffUser = { username: 'anonymous', signIn: false }

describe('route guards', () => {
  beforeEach(() => {
    requestedPaths = []
    useAuthStore.getState().auth.reset()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('pages behind sign-in', () => {
    // A link to a file opened without a session: sign in first, then land on that same file.
    it('sends a visitor without a session to sign-in with the way back', async () => {
      serveApi({ setupRequired: false, currentUser: 401 })
      const { router, screen } = await openApp('/env-files/api.env?x=1')

      await expect.element(screen.getByText('sign-in page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/sign-in')
      expect(router.state.location.search).toEqual({
        redirect: '/env-files/api.env?x=1',
      })
      expect(screen.getByText('env file page').query()).toBeNull()
    })

    // A fresh install has no account yet: signing in is impossible, so setup comes first,
    // without even asking who the visitor is.
    it('sends everyone to setup before the first account exists', async () => {
      serveApi({ setupRequired: true, currentUser: 401 })
      const { router, screen } = await openApp('/env-files')

      await expect.element(screen.getByText('setup page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/setup')
      expect(meRequests()).toBe(0)
    })

    // Without sign-in the pages open directly, and the red banner reminds that anyone can read them.
    it('opens pages directly with a warning while sign-in is off', async () => {
      serveApi({ setupRequired: false, currentUser: signInOffUser })
      const { router, screen } = await openApp('/env-files')

      await expect.element(screen.getByText('env files page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/env-files')
      expect(useAuthStore.getState().auth.user).toEqual(signInOffUser)
      await expect.element(screen.getByText('No sign-in:')).toBeVisible()
    })

    // Who is signed in is asked once; moving between pages must not call the server each time.
    it('asks who is signed in only once', async () => {
      serveApi({ setupRequired: false, currentUser: signedInAdmin })
      const { router, screen } = await openApp('/env-files')
      await expect.element(screen.getByText('env files page')).toBeVisible()
      expect(useAuthStore.getState().auth.user).toEqual(signedInAdmin)
      expect(screen.getByText('No sign-in:').query()).toBeNull()

      await router.navigate({
        to: '/env-files/$name',
        params: { name: 'api.env' },
      })
      await expect.element(screen.getByText('env file page')).toBeVisible()
      expect(meRequests()).toBe(1)
    })

    // A broken server is not "signed out": show the error page instead of a sign-in form
    // that could never work.
    it('shows the error page when the server cannot say who is signed in', async () => {
      serveApi({ setupRequired: false, currentUser: 500 })
      const { router, screen } = await openApp('/env-files')

      await expect
        .element(screen.getByText('Oops! Something went wrong'))
        .toBeVisible()
      expect(router.state.location.pathname).toBe('/env-files')
      expect(screen.getByText('env files page').query()).toBeNull()
    })
  })

  describe('sign-in page', () => {
    it('sends to setup before the first account exists', async () => {
      serveApi({ setupRequired: true, currentUser: 401 })
      const { router, screen } = await openApp('/sign-in')

      await expect.element(screen.getByText('setup page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/setup')
    })

    // There is nothing to sign in to while sign-in is off.
    it('is skipped while sign-in is off', async () => {
      serveApi({ setupRequired: false, currentUser: signInOffUser })
      const { router, screen } = await openApp('/sign-in')

      await expect.element(screen.getByText('env files page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/env-files')
    })

    // The form must keep the way back, or signing in lands on the home page instead of the file.
    it('shows the form and keeps the way back when there is no session', async () => {
      serveApi({ setupRequired: false, currentUser: 401 })
      const { router, screen } = await openApp(
        '/sign-in?redirect=%2Fenv-files%2Fapi.env'
      )

      await expect.element(screen.getByText('sign-in page')).toBeVisible()
      expect(router.state.location.search).toEqual({
        redirect: '/env-files/api.env',
      })
    })
  })

  describe('setup page', () => {
    // Once the admin exists the setup page is gone for good: a second admin cannot be created.
    it('sends to sign-in once the first account exists', async () => {
      serveApi({ setupRequired: false, currentUser: 401 })
      const { router, screen } = await openApp('/setup')

      await expect.element(screen.getByText('sign-in page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/sign-in')
    })

    // Setup done without sign-in: the chain setup → sign-in → home ends on the env files.
    it('ends on the env files when setup was done without sign-in', async () => {
      serveApi({ setupRequired: false, currentUser: signInOffUser })
      const { router, screen } = await openApp('/setup')

      await expect.element(screen.getByText('env files page')).toBeVisible()
      expect(router.state.location.pathname).toBe('/env-files')
    })
  })
})
