import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { FolderOpen, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { saveEnvFolder } from '../api/env-files-api'

type EnvFolderFormProps = {
  savedFolder: string
}

// The folder is a path inside the container; the server checks it exists before saving.
export function EnvFolderForm({ savedFolder }: EnvFolderFormProps) {
  const [folder, setFolder] = useState(savedFolder)
  const queryClient = useQueryClient()
  const saveFolder = useMutation({
    mutationFn: saveEnvFolder,
    // Everything shown so far came from the old folder.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['env'] }),
  })

  return (
    <form
      className='grid gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        saveFolder.mutate(folder.trim())
      }}
    >
      <Label htmlFor='env-folder'>Env folder</Label>
      <div className='flex gap-2'>
        <Input
          id='env-folder'
          placeholder='/host/env'
          className='font-mono'
          value={folder}
          onChange={(event) => setFolder(event.target.value)}
        />
        <Button
          type='submit'
          disabled={saveFolder.isPending || folder.trim() === savedFolder}
        >
          {saveFolder.isPending ? (
            <Loader2 className='animate-spin' />
          ) : (
            <FolderOpen />
          )}
          Save
        </Button>
      </div>
      <p className='text-sm text-muted-foreground'>
        Path inside the container. Mount the host folder first, e.g.{' '}
        <code className='font-mono'>/opt/textiq/env:/host/env</code>.
      </p>
      {saveFolder.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {saveFolder.error.message}
        </p>
      )}
    </form>
  )
}
