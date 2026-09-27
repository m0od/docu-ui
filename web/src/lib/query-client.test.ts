import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import { ApiError } from './api-client'
import { createQueryClient } from './query-client'

// A router shaped like the app's, served under a gateway sub-path as with DOCU_BASE_PATH.
async function appUnderBasePath(startPath: string) {
  const rootRoute = createRootRoute()
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      createRoute({ getParentRoute: () => rootRoute, path: '/sign-in' }),
      createRoute({
        getParentRoute: () => rootRoute,
        path: '/env-files/$name',
      }),
    ]),
    history: createMemoryHistory({ initialEntries: [startPath] }),
    basepath: '/docu-ui/',
  })
  await router.load()
  return { router, queryClient: createQueryClient(() => router) }
}

function failingQuery(
  queryClient: ReturnType<typeof createQueryClient>,
  queryName: string,
  status: number
) {
  return queryClient
    .fetchQuery({
      queryKey: [queryName],
      queryFn: () => Promise.reject(new ApiError(status, {})),
    })
    .catch(() => {})
}

describe('createQueryClient', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setUser({ username: 'admin', signIn: true })
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })

  // A session that expired or was signed out elsewhere: sign in again and come back to the same page.
  // The return path must not carry the base path, or the router adds it twice (/docu-ui/docu-ui/...).
  it('sends a 401 to sign-in and comes back to the same page under the base path', async () => {
    const { router, queryClient } = await appUnderBasePath(
      '/docu-ui/env-files/api.env?tab=history'
    )
    await failingQuery(queryClient, 'env-file', 401)
    await vi.waitFor(() =>
      expect(router.history.location.pathname).toBe('/docu-ui/sign-in')
    )
    const redirect = (router.latestLocation.search as { redirect: string })
      .redirect
    expect(redirect).toBe('/env-files/api.env?tab=history')
    expect(useAuthStore.getState().auth.user).toBeNull()

    // What the sign-in form does after a successful sign-in.
    await router.navigate({ to: redirect })
    expect(router.history.location.pathname).toBe('/docu-ui/env-files/api.env')
  })

  // A file page runs several queries at once; if each redirected, the second would use the
  // sign-in page as the return path and the user would land back on sign-in.
  it('redirects once when several queries fail together', async () => {
    const { router, queryClient } = await appUnderBasePath(
      '/docu-ui/env-files/api.env'
    )
    const navigateSpy = vi.spyOn(router, 'navigate')
    await Promise.all([
      failingQuery(queryClient, 'env-file', 401),
      failingQuery(queryClient, 'apply-target', 401),
    ])
    await vi.waitFor(() =>
      expect(router.history.location.pathname).toBe('/docu-ui/sign-in')
    )
    expect(navigateSpy).toHaveBeenCalledTimes(1)
    expect(router.latestLocation.search).toEqual({
      redirect: '/env-files/api.env',
    })
  })

  // Only a missing session means signing in again; other errors are shown where they happen.
  it.each([403, 404, 500])('stays on the page for a %i', async (status) => {
    const { router, queryClient } = await appUnderBasePath(
      '/docu-ui/env-files/api.env'
    )
    await failingQuery(queryClient, 'env-file', status)
    expect(router.history.location.pathname).toBe('/docu-ui/env-files/api.env')
    expect(useAuthStore.getState().auth.user).not.toBeNull()
  })

  // A failed save keeps the page, so unsaved edits survive a 401 (sign in in another tab, save again),
  // and each form shows its own error: no generic "Something went wrong!" toast on top.
  it('leaves failed saves to the form that made them', async () => {
    const { router, queryClient } = await appUnderBasePath(
      '/docu-ui/env-files/api.env'
    )
    const toastSpy = vi.spyOn(toast, 'error')
    for (const status of [400, 401, 500]) {
      await queryClient
        .getMutationCache()
        .build(queryClient, {
          mutationFn: () => Promise.reject(new ApiError(status, {})),
        })
        .execute(undefined)
        .catch(() => {})
    }
    expect(toastSpy).not.toHaveBeenCalled()
    expect(router.history.location.pathname).toBe('/docu-ui/env-files/api.env')
    expect(useAuthStore.getState().auth.user).not.toBeNull()
  })

  // A 4xx will not change on retry; a 5xx may be a restart in progress, so production tries a few times.
  it('retries only server errors, only in production', async () => {
    const { queryClient } = await appUnderBasePath('/docu-ui/env-files/api.env')
    const retry = queryClient.getDefaultOptions().queries?.retry as (
      failureCount: number,
      error: Error
    ) => boolean
    vi.stubEnv('DEV', false)
    expect(retry(1, new ApiError(404, {}))).toBe(false)
    expect(retry(1, new ApiError(503, {}))).toBe(true)
    expect(retry(1, new TypeError('Failed to fetch'))).toBe(true)
    expect(retry(4, new ApiError(503, {}))).toBe(false)
    // In development an error shows at once instead of after the retries.
    vi.stubEnv('DEV', true)
    expect(retry(1, new ApiError(503, {}))).toBe(false)
  })
})
