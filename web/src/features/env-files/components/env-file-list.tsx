import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { FileText } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { listEnvFiles } from '../api/env-files-api'

export function EnvFileList() {
  const envFiles = useQuery({
    queryKey: ['env', 'files'],
    queryFn: listEnvFiles,
  })

  if (envFiles.isPending) {
    return <Skeleton className='h-24 w-full' />
  }
  if (envFiles.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {envFiles.error.message}
      </p>
    )
  }
  if (envFiles.data.length === 0) {
    return (
      <p className='text-sm text-muted-foreground'>
        No <code className='font-mono'>.env</code> files in this folder.
      </p>
    )
  }
  return (
    <ul className='divide-y rounded-md border'>
      {envFiles.data.map((fileName) => (
        <li key={fileName}>
          <Link
            to='/env-files/$name'
            params={{ name: fileName }}
            className='flex items-center gap-2 px-4 py-3 font-mono text-sm hover:bg-muted'
          >
            <FileText className='size-4 text-muted-foreground' />
            {fileName}
          </Link>
        </li>
      ))}
    </ul>
  )
}
