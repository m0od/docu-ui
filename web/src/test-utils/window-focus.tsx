import {
  QueryClientProvider,
  focusManager,
  useQuery,
} from '@tanstack/react-query'
import { expect, vi } from 'vitest'
import { createQueryClient } from '@/lib/query-client'

// An ordinary query next to the one under test: when it is fetched again, a focus refetch has run.
const probeQueryFn = vi.fn(() => Promise.resolve('probe'))

function ProbeQuery() {
  useQuery({ queryKey: ['probe'], queryFn: probeQueryFn })
  return null
}

// withProductionQueryClient renders children with the app's own cache settings as built for
// production, where the window regaining focus refetches stale queries.
export function withProductionQueryClient() {
  vi.stubEnv('PROD', true)
  const queryClient = createQueryClient(() => {
    throw new Error('no router in this test')
  })
  return (children: React.ReactNode) => (
    <QueryClientProvider client={queryClient}>
      <ProbeQuery />
      {children}
    </QueryClientProvider>
  )
}

// returnToTabLater simulates coming back to the tab a minute later, and waits until the probe
// query has been refetched, so what was not refetched by then never will be.
export async function returnToTabLater() {
  await vi.waitFor(() => expect(probeQueryFn).toHaveBeenCalled())
  const probeCalls = probeQueryFn.mock.calls.length
  const minuteLater = Date.now() + 60 * 1000
  vi.spyOn(Date, 'now').mockReturnValue(minuteLater)
  focusManager.setFocused(false)
  focusManager.setFocused(true)
  await vi.waitFor(() =>
    expect(probeQueryFn).toHaveBeenCalledTimes(probeCalls + 1)
  )
  focusManager.setFocused(undefined)
}
