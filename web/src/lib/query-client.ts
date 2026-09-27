import { QueryCache, QueryClient } from '@tanstack/react-query'
import type { AnyRouter } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ApiError } from '@/lib/api-client'

// createQueryClient builds the app's cache. getRouter is a function because the
// router is created after the client (it takes the client as context).
export function createQueryClient(getRouter: () => AnyRouter) {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => {
          if (import.meta.env.DEV) return false
          // A 4xx from our API will not change on retry (bad name, no folder, no session).
          if (error instanceof ApiError && error.status < 500) return false
          return failureCount <= 3
        },
        refetchOnWindowFocus: import.meta.env.PROD,
        staleTime: 10 * 1000, // 10s
      },
      // No global mutation handler: each form shows its own error next to the
      // button, and a 401 there keeps the page so unsaved edits are not lost.
    },
    queryCache: new QueryCache({
      onError: (error) => {
        if (error instanceof ApiError && error.status === 401) {
          sendToSignIn(getRouter())
        }
      },
    }),
  })
}

// sendToSignIn handles a session that expired or was signed out elsewhere:
// back to sign-in, then return to the same page.
function sendToSignIn(router: AnyRouter) {
  const location = router.latestLocation
  // Several queries on one page fail together; only the first one redirects,
  // or the return path would become the sign-in page itself.
  if (location.pathname === '/sign-in') {
    return
  }
  useAuthStore.getState().auth.reset()
  // href is the path inside the app, without the gateway base path: the
  // router adds the base path again when the sign-in form navigates back.
  router.navigate({ to: '/sign-in', search: { redirect: location.href } })
}
