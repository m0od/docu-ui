import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { EnvFileView } from './env-file-view'

// The real page with the real editor, against a fake server: the bug lived between the two.
let openFileName = 'api.env'
vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useParams: () => ({ name: openFileName }),
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}))
vi.mock('./components/apply-panel', () => ({
  ApplyPanel: () => null,
}))
vi.mock('@/components/layout/header', () => ({
  Header: () => null,
}))
vi.mock('@/components/layout/main', () => ({
  Main: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}))

type FakeEnvFile = {
  version: string
  values: Record<string, string>
}

let serverFiles: Record<string, FakeEnvFile> = {}
let savedChanges: unknown[] = []

function serveEnvFiles() {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const path = decodeURIComponent(
      new URL(String(input), document.baseURI).pathname
    )
    const [, fileName, , key] =
      path.match(/\/api\/env-files\/([^/]+)(\/variables(?:\/(.+))?)?$/) ?? []
    const file = serverFiles[fileName]
    if (init?.method === 'PATCH') {
      const body = JSON.parse(String(init.body))
      if (body.baseVersion !== file.version) {
        return Response.json(
          {
            error: 'the file changed since you opened it: reload and try again',
          },
          { status: 409 }
        )
      }
      savedChanges.push(body)
      return new Response(null, { status: 204 })
    }
    if (key) {
      return Response.json({ value: file.values[key] })
    }
    return Response.json({
      name: fileName,
      version: file.version,
      variables: Object.keys(file.values).map((variableKey, index) => ({
        key: variableKey,
        line: index + 1,
        overridden: false,
      })),
      invalidLines: [],
    })
  })
}

async function saveAndMeetConflict(screen: Awaited<ReturnType<typeof render>>) {
  await userEvent.click(screen.getByRole('button', { name: 'Review and save' }))
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  await expect
    .element(screen.getByRole('alert'))
    .toHaveTextContent('the file changed since you opened it')
}

describe('EnvFileView reload after a conflict', () => {
  beforeEach(() => {
    openFileName = 'api.env'
    savedChanges = []
    serverFiles = {
      'api.env': {
        version: 'version-1',
        values: { APP_DB: 'postgres', APP_MODE: 'blue' },
      },
      'worker.env': { version: 'version-1', values: { APP_QUEUE: 'jobs' } },
    }
    serveEnvFiles()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // Someone saved while the user was editing. "Reload file" must bring the new version
  // and keep the user's unsaved changes, so they can be saved on top of it, not typed again.
  it('keeps unsaved changes and saves them on the new version', async () => {
    const screen = await render(withQueryClient(<EnvFileView />))
    await userEvent.click(screen.getByRole('button', { name: 'Remove APP_DB' }))
    serverFiles['api.env'] = {
      version: 'version-2',
      values: { APP_DB: 'postgres', APP_MODE: 'blue', APP_CACHE: 'on' },
    }

    await saveAndMeetConflict(screen)
    await userEvent.click(screen.getByRole('button', { name: 'Reload file' }))

    await expect.element(screen.getByText('APP_CACHE')).toBeVisible()
    await expect.element(screen.getByText('1 unsaved change')).toBeVisible()
    await expect
      .element(screen.getByText('removed', { exact: true }))
      .toBeVisible()

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await vi.waitFor(() =>
      expect(savedChanges).toEqual([
        { baseVersion: 'version-2', changes: [{ key: 'APP_DB', value: null }] },
      ])
    )
  })

  // A value shown before the reload may have been changed by the other save:
  // it is hidden again instead of showing an old value as if it were current.
  it('hides values shown from the old version', async () => {
    const screen = await render(withQueryClient(<EnvFileView />))
    await userEvent.click(screen.getByRole('button', { name: 'Show APP_MODE' }))
    await expect.element(screen.getByText('blue')).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Remove APP_DB' }))
    serverFiles['api.env'] = {
      version: 'version-2',
      values: { APP_DB: 'postgres', APP_MODE: 'green' },
    }

    await saveAndMeetConflict(screen)
    await userEvent.click(screen.getByRole('button', { name: 'Reload file' }))
    await userEvent.keyboard('{Escape}')

    await expect
      .element(screen.getByRole('button', { name: 'Show APP_MODE' }))
      .toBeVisible()
    expect(screen.getByText('blue').query()).toBeNull()
  })

  // Unsaved changes belong to one file: opening another file must not carry them over,
  // or saving there would remove APP_DB from the wrong file. The other file was opened
  // before, so its cached content shows at once and nothing else resets the editor.
  it('drops unsaved changes when another file is opened', async () => {
    // One cache for both renders, as in the app: a new provider would remount everything anyway.
    const queryClient = new QueryClient()
    const page = () => (
      <QueryClientProvider client={queryClient}>
        <EnvFileView />
      </QueryClientProvider>
    )
    openFileName = 'worker.env'
    const screen = await render(page())
    await expect.element(screen.getByText('APP_QUEUE')).toBeVisible()
    openFileName = 'api.env'
    await screen.rerender(page())
    await userEvent.click(screen.getByRole('button', { name: 'Remove APP_DB' }))
    await expect.element(screen.getByText('1 unsaved change')).toBeVisible()

    openFileName = 'worker.env'
    await screen.rerender(page())

    await expect.element(screen.getByText('APP_QUEUE')).toBeVisible()
    expect(screen.getByText('1 unsaved change').query()).toBeNull()
  })
})
