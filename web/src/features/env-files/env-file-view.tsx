import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { ArrowLeft, FileCode, History, TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfigDrawer } from '@/components/config-drawer'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { readEnvFile } from './api/env-files-api'
import { ApplyPanel } from './components/apply-panel'
import { EnvHistory } from './components/env-history'
import { EnvTextEditor } from './components/env-text-editor'
import { EnvVariablesEditor } from './components/env-variables-editor'

type ViewMode = 'variables' | 'text' | 'history'

export function EnvFileView() {
  const { name: fileName } = useParams({
    from: '/_authenticated/env-files/$name',
  })
  const [viewMode, setViewMode] = useState<ViewMode>('variables')
  // Text and history show every value, so they wait for the user to agree;
  // 'variables' here means no warning is open.
  const [modeAwaitingConsent, setModeAwaitingConsent] =
    useState<ViewMode>('variables')
  const envFile = useQuery({
    queryKey: ['env', 'file', fileName],
    queryFn: () => readEnvFile(fileName),
  })

  return (
    <>
      <Header>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      <Main>
        <Link
          to='/env-files'
          className='mb-2 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground'
        >
          <ArrowLeft className='size-4' />
          Env files
        </Link>
        <h1 className='mb-4 font-mono text-2xl font-bold tracking-tight'>
          {fileName}
        </h1>

        {envFile.isPending && <Skeleton className='h-40 w-full' />}
        {envFile.isError && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {envFile.error.message}
          </p>
        )}
        {envFile.isSuccess && (
          <div className='grid max-w-5xl gap-4'>
            {envFile.data.invalidLines.length > 0 && (
              <Alert variant='destructive'>
                <TriangleAlert />
                <AlertTitle>Lines Compose cannot read</AlertTitle>
                <AlertDescription>
                  {envFile.data.invalidLines.length === 1
                    ? `Line ${envFile.data.invalidLines[0]} is`
                    : `Lines ${envFile.data.invalidLines.join(', ')} are`}{' '}
                  neither a comment nor KEY=value.
                </AlertDescription>
              </Alert>
            )}
            {viewMode === 'text' && (
              <EnvTextEditor
                fileName={fileName}
                onClose={() => setViewMode('variables')}
              />
            )}
            {viewMode === 'history' && (
              <EnvHistory
                fileName={fileName}
                onClose={() => setViewMode('variables')}
              />
            )}
            {viewMode === 'variables' && (
              <>
                <ApplyPanel
                  fileName={fileName}
                  version={envFile.data.version}
                />
                <div className='flex justify-end gap-2'>
                  <Button
                    variant='outline'
                    onClick={() => setModeAwaitingConsent('history')}
                  >
                    <History />
                    History
                  </Button>
                  <Button
                    variant='outline'
                    onClick={() => setModeAwaitingConsent('text')}
                  >
                    <FileCode />
                    Edit as text
                  </Button>
                </div>
                <EnvVariablesEditor
                  // A new version (after a save) starts a fresh editor.
                  key={envFile.data.version}
                  fileName={fileName}
                  version={envFile.data.version}
                  variables={envFile.data.variables}
                />
              </>
            )}
          </div>
        )}
        <ConfirmDialog
          open={modeAwaitingConsent !== 'variables'}
          onOpenChange={() => setModeAwaitingConsent('variables')}
          title='Show every value?'
          desc='This shows all values of this file in clear, including passwords. Make sure nobody else can see your screen.'
          confirmText={
            modeAwaitingConsent === 'history' ? 'Show history' : 'Show and edit'
          }
          handleConfirm={() => {
            setModeAwaitingConsent('variables')
            setViewMode(modeAwaitingConsent)
          }}
        />
      </Main>
    </>
  )
}
