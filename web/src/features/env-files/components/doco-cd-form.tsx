import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Save } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchDocoCD, saveDocoCD } from '../api/apply-api'

// Where Apply sends its recreate calls. Set once for all files.
export function DocoCDForm() {
  const settings = useQuery({
    queryKey: ['env', 'doco-cd'],
    queryFn: fetchDocoCD,
  })

  if (settings.isPending) {
    return <Skeleton className='h-32 w-full' />
  }
  if (settings.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {settings.error.message}
      </p>
    )
  }
  return (
    <DocoCDFields
      savedUrl={settings.data.url}
      hasApiKey={settings.data.hasApiKey}
    />
  )
}

type DocoCDFieldsProps = {
  savedUrl: string
  hasApiKey: boolean
}

function DocoCDFields({ savedUrl, hasApiKey }: DocoCDFieldsProps) {
  const [url, setUrl] = useState(savedUrl)
  const [apiKey, setApiKey] = useState('')
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: () => saveDocoCD(url.trim(), apiKey),
    onSuccess: (saved) => {
      setApiKey('')
      queryClient.setQueryData(['env', 'doco-cd'], saved)
    },
  })

  return (
    <form
      className='grid gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate()
      }}
    >
      <Label htmlFor='doco-cd-url'>Doco-CD URL</Label>
      <Input
        id='doco-cd-url'
        placeholder='http://doco-cd:80'
        className='font-mono'
        value={url}
        onChange={(event) => setUrl(event.target.value)}
      />
      <Label htmlFor='doco-cd-api-key'>Doco-CD API key</Label>
      <div className='flex gap-2'>
        <Input
          id='doco-cd-api-key'
          type='password'
          autoComplete='off'
          placeholder={
            hasApiKey
              ? 'Saved; leave empty to keep it'
              : 'API_SECRET of Doco-CD'
          }
          value={apiKey}
          onChange={(event) => setApiKey(event.target.value)}
        />
        <Button
          type='submit'
          disabled={
            save.isPending || (url.trim() === savedUrl && apiKey === '')
          }
        >
          {save.isPending ? <Loader2 className='animate-spin' /> : <Save />}
          Save
        </Button>
      </div>
      <p className='text-sm text-muted-foreground'>
        Apply asks Doco-CD to recreate the services that use a file. Docu-UI
        must reach this URL (e.g. same Docker network), and Doco-CD needs{' '}
        <code className='font-mono'>API_SECRET</code> set to enable its API.
      </p>
      {save.isSuccess && (
        <p className='text-sm text-muted-foreground'>Doco-CD settings saved.</p>
      )}
      {save.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {save.error.message}
        </p>
      )}
    </form>
  )
}
