import { useQuery } from '@tanstack/react-query'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchEnvFolder } from '@/features/env-files/api/env-files-api'
import { EnvFolderForm } from '@/features/env-files/components/env-folder-form'
import { ContentSection } from '../components/content-section'

export function SettingsEnvFolder() {
  const envFolder = useQuery({
    queryKey: ['env', 'folder'],
    queryFn: fetchEnvFolder,
  })

  return (
    <ContentSection
      title='Env folder'
      desc='Where Docu-UI finds the env files it lists and edits.'
    >
      <>
        {envFolder.isPending && <Skeleton className='h-20 w-full' />}
        {envFolder.isError && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {envFolder.error.message}
          </p>
        )}
        {envFolder.isSuccess && (
          // key: a saved change re-seeds the input with the new folder
          <EnvFolderForm key={envFolder.data} savedFolder={envFolder.data} />
        )}
      </>
    </ContentSection>
  )
}
