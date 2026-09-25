import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { WebhookInput, WebhookView } from '../api/apply-api'

type WebhookFieldsProps = {
  // Keeps input ids unique when two sets of fields are on one page.
  idPrefix: string
  value: WebhookInput
  // What is saved now, to say which secrets an empty field keeps.
  saved: WebhookView | null
  onChange: (value: WebhookInput) => void
}

const KEEP_SAVED = 'Saved; leave empty to keep it'

export function WebhookFields({
  idPrefix,
  value,
  saved,
  onChange,
}: WebhookFieldsProps) {
  return (
    <div className='grid gap-2 sm:grid-cols-2'>
      <div className='grid gap-1 sm:col-span-2'>
        <Label htmlFor={`${idPrefix}-url`}>Webhook URL</Label>
        <Input
          id={`${idPrefix}-url`}
          placeholder='https://api.github.com/repos/owner/repo/dispatches'
          className='font-mono'
          value={value.url}
          onChange={(event) => onChange({ ...value, url: event.target.value })}
        />
      </div>
      <div className='grid gap-1 sm:col-span-2'>
        <Label htmlFor={`${idPrefix}-secret`}>Signing secret</Label>
        <Input
          id={`${idPrefix}-secret`}
          type='password'
          autoComplete='off'
          placeholder={
            saved?.hasSecret
              ? KEEP_SAVED
              : 'Optional: signs the body with HMAC-SHA256'
          }
          value={value.secret}
          onChange={(event) =>
            onChange({ ...value, secret: event.target.value })
          }
        />
      </div>
      <div className='grid gap-1'>
        <Label htmlFor={`${idPrefix}-header-name`}>Header name</Label>
        <Input
          id={`${idPrefix}-header-name`}
          placeholder='Optional: Authorization'
          className='font-mono'
          value={value.headerName}
          onChange={(event) =>
            onChange({ ...value, headerName: event.target.value })
          }
        />
      </div>
      <div className='grid gap-1'>
        <Label htmlFor={`${idPrefix}-header-value`}>Header value</Label>
        <Input
          id={`${idPrefix}-header-value`}
          type='password'
          autoComplete='off'
          placeholder={saved?.hasHeaderValue ? KEEP_SAVED : 'Bearer …'}
          value={value.headerValue}
          onChange={(event) =>
            onChange({ ...value, headerValue: event.target.value })
          }
        />
      </div>
    </div>
  )
}
