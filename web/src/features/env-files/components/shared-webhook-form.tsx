import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Save } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  fetchSharedWebhook,
  saveSharedWebhook,
  type WebhookInput,
  type WebhookView,
} from '../api/apply-api'
import { WebhookFields } from './webhook-fields'

// The webhook files use unless they have their own. Set once.
export function SharedWebhookForm() {
  const sharedWebhook = useQuery({
    queryKey: ['env', 'webhook'],
    queryFn: fetchSharedWebhook,
  })

  if (sharedWebhook.isPending) {
    return <Skeleton className='h-40 w-full' />
  }
  if (sharedWebhook.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {sharedWebhook.error.message}
      </p>
    )
  }
  return <SharedWebhookFields saved={sharedWebhook.data} />
}

function SharedWebhookFields({ saved }: { saved: WebhookView }) {
  const [webhook, setWebhook] = useState<WebhookInput>({
    url: saved.url,
    secret: '',
    headerName: saved.headerName,
    headerValue: '',
  })
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: () =>
      saveSharedWebhook({ ...webhook, url: webhook.url.trim() }),
    onSuccess: (savedWebhook) => {
      setWebhook({ ...webhook, secret: '', headerValue: '' })
      queryClient.setQueryData(['env', 'webhook'], savedWebhook)
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
      <p className='text-sm font-medium'>Shared webhook</p>
      <WebhookFields
        idPrefix='shared-webhook'
        value={webhook}
        saved={saved}
        onChange={setWebhook}
      />
      <p className='text-sm text-muted-foreground'>
        Apply posts{' '}
        <code className='font-mono'>
          {
            '{"event_type": "docu-ui.apply", "client_payload": {file, version, project, services, user, sentAt}}'
          }
        </code>{' '}
        (no values), the shape GitHub <code>repository_dispatch</code> expects.
        With a secret, <code className='font-mono'>X-Docu-UI-Signature</code> is{' '}
        <code className='font-mono'>sha256=</code> HMAC of the body.
      </p>
      {save.isSuccess && (
        <p className='text-sm text-muted-foreground'>Webhook saved.</p>
      )}
      {save.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {save.error.message}
        </p>
      )}
      <div className='flex justify-end'>
        <Button
          type='submit'
          disabled={save.isPending || webhook.url.trim() === ''}
        >
          {save.isPending ? <Loader2 className='animate-spin' /> : <Save />}
          Save
        </Button>
      </div>
    </form>
  )
}
