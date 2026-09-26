import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { fetchEnvFolder } from './api/env-files-api'
import { EnvFileList } from './components/env-file-list'

export function EnvFiles() {
  const envFolder = useQuery({
    queryKey: ['env', 'folder'],
    queryFn: fetchEnvFolder,
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
        <div className='space-y-0.5'>
          <h1 className='text-2xl font-bold tracking-tight'>Env files</h1>
          <p className='text-muted-foreground'>
            The <code className='font-mono'>.env</code> files your Compose
            services read.
          </p>
        </div>
        <Separator className='my-4' />
        <div className='grid max-w-3xl gap-6'>
          {envFolder.isPending && <Skeleton className='h-20 w-full' />}
          {envFolder.isError && (
            <p role='alert' className='text-sm font-medium text-destructive'>
              {envFolder.error.message}
            </p>
          )}
          {envFolder.isSuccess &&
            (envFolder.data === '' ? (
              <p className='text-sm text-muted-foreground'>
                <Link to='/settings/env-folder' className='underline'>
                  Choose the folder
                </Link>{' '}
                that holds your env files to get started.
              </p>
            ) : (
              <>
                <p className='text-sm text-muted-foreground'>
                  In <code className='font-mono'>{envFolder.data}</code> ·{' '}
                  <Link to='/settings/env-folder' className='underline'>
                    Change
                  </Link>
                </p>
                <EnvFileList />
              </>
            ))}
        </div>
      </Main>
    </>
  )
}
