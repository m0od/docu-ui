import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { ArrowLeft, TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { readEnvFile } from './api/env-files-api'
import { EnvVariableRow } from './components/env-variable-row'

export function EnvFileView() {
  const { name: fileName } = useParams({
    from: '/_authenticated/env-files/$name',
  })
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
          <div className='grid max-w-4xl gap-4'>
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
            {envFile.data.variables.length === 0 ? (
              <p className='text-sm text-muted-foreground'>
                This file sets no variables.
              </p>
            ) : (
              <div className='rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Line</TableHead>
                      <TableHead>Key</TableHead>
                      <TableHead>Value</TableHead>
                      <TableHead>
                        <span className='sr-only'>Show value</span>
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {envFile.data.variables.map((variable) => (
                      <EnvVariableRow
                        key={variable.line}
                        fileName={fileName}
                        variable={variable}
                      />
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        )}
      </Main>
    </>
  )
}
