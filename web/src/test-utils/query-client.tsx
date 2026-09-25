import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// withQueryClient gives each test its own cache, with no retries so an error shows at once.
export function withQueryClient(children: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}
