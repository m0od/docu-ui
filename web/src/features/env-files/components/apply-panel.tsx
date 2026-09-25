import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleCheck, Rocket, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  applyEnvFile,
  fetchApplyState,
  saveApplyTarget,
  type ApplyAdapter,
  type ApplyTarget,
  type WebhookInput,
} from '../api/apply-api'
import { WebhookFields } from './webhook-fields'

type ApplyPanelProps = {
  fileName: string
  // The file version on screen; Apply sends exactly this one live.
  version: string
}

// Which services use this file, and whether its saved content reached them yet.
export function ApplyPanel({ fileName, version }: ApplyPanelProps) {
  const [isEditing, setIsEditing] = useState(false)
  const applyState = useQuery({
    queryKey: ['env', 'apply-target', fileName],
    queryFn: () => fetchApplyState(fileName),
  })

  if (applyState.isPending) {
    return <Skeleton className='h-16 w-full' />
  }
  if (applyState.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {applyState.error.message}
      </p>
    )
  }
  const { target, appliedVersion } = applyState.data
  if (target === null || isEditing) {
    return (
      <ApplyTargetForm
        fileName={fileName}
        savedTarget={target}
        onDone={() => setIsEditing(false)}
      />
    )
  }
  return (
    <div className='grid gap-2'>
      <div className='flex flex-wrap items-center gap-2 text-sm'>
        <span>
          Applies via {describeAdapter(target)} to{' '}
          <code className='font-mono'>{describeServices(target)}</code>
        </span>
        <Button
          variant='link'
          className='h-auto p-0'
          onClick={() => setIsEditing(true)}
        >
          Change
        </Button>
        {version === appliedVersion && (
          <span className='ms-auto inline-flex items-center gap-1 text-muted-foreground'>
            <CircleCheck className='size-4' />
            Applied
          </span>
        )}
      </div>
      {version !== appliedVersion && (
        <NotAppliedAlert
          fileName={fileName}
          version={version}
          target={target}
        />
      )}
    </div>
  )
}

type ApplyTargetFormProps = {
  fileName: string
  savedTarget: ApplyTarget | null
  onDone: () => void
}

function ApplyTargetForm({
  fileName,
  savedTarget,
  onDone,
}: ApplyTargetFormProps) {
  const [adapter, setAdapter] = useState<ApplyAdapter>(
    savedTarget?.adapter ?? 'doco-cd'
  )
  const savedOwnWebhook =
    savedTarget !== null && savedTarget.webhook.url !== ''
      ? savedTarget.webhook
      : null
  const [hasOwnWebhook, setHasOwnWebhook] = useState(savedOwnWebhook !== null)
  const [ownWebhook, setOwnWebhook] = useState<WebhookInput>({
    url: savedOwnWebhook?.url ?? '',
    secret: '',
    headerName: savedOwnWebhook?.headerName ?? '',
    headerValue: '',
  })
  const [project, setProject] = useState(savedTarget?.project ?? '')
  const [services, setServices] = useState(
    savedTarget?.services.join(' ') ?? ''
  )
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: () =>
      saveApplyTarget(fileName, {
        adapter,
        project: project.trim(),
        services: services.split(/[\s,]+/).filter(Boolean),
        // An empty URL means the shared webhook.
        webhook:
          adapter === 'webhook' && hasOwnWebhook
            ? { ...ownWebhook, url: ownWebhook.url.trim() }
            : NO_OWN_WEBHOOK,
      }),
    onSuccess: (saved) => {
      queryClient.setQueryData(['env', 'apply-target', fileName], saved)
      onDone()
    },
  })

  // Doco-CD needs a project; an own webhook needs its URL.
  const isComplete =
    adapter === 'doco-cd'
      ? project.trim() !== ''
      : !hasOwnWebhook || ownWebhook.url.trim() !== ''

  return (
    <form
      className='grid gap-2 rounded-md border p-3'
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate()
      }}
    >
      <p className='text-sm font-medium'>How is this file applied?</p>
      <RadioGroup
        className='flex gap-4'
        value={adapter}
        onValueChange={(value) => setAdapter(value as ApplyAdapter)}
      >
        <Label className='font-normal'>
          <RadioGroupItem value='doco-cd' />
          Doco-CD
        </Label>
        <Label className='font-normal'>
          <RadioGroupItem value='webhook' />
          Webhook to your CI/CD
        </Label>
      </RadioGroup>
      {adapter === 'webhook' && (
        <>
          <Label className='font-normal'>
            <Checkbox
              checked={hasOwnWebhook}
              onCheckedChange={(checked) => setHasOwnWebhook(checked === true)}
            />
            Use its own webhook instead of the shared one
          </Label>
          {hasOwnWebhook && (
            <WebhookFields
              idPrefix='own-webhook'
              value={ownWebhook}
              saved={savedOwnWebhook}
              onChange={setOwnWebhook}
            />
          )}
        </>
      )}
      <div className='grid gap-2 sm:grid-cols-2'>
        <div className='grid gap-1'>
          <Label htmlFor='apply-project'>
            {adapter === 'webhook'
              ? 'Compose project (optional)'
              : 'Compose project'}
          </Label>
          <Input
            id='apply-project'
            placeholder='textiq-dev'
            className='font-mono'
            value={project}
            onChange={(event) => setProject(event.target.value)}
          />
        </div>
        <div className='grid gap-1'>
          <Label htmlFor='apply-services'>Services</Label>
          <Input
            id='apply-services'
            placeholder='keycloak iam (empty: whole project)'
            className='font-mono'
            value={services}
            onChange={(event) => setServices(event.target.value)}
          />
        </div>
      </div>
      {save.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {save.error.message}
        </p>
      )}
      <div className='flex justify-end gap-2'>
        {savedTarget !== null && (
          <Button type='button' variant='outline' onClick={onDone}>
            Cancel
          </Button>
        )}
        <Button type='submit' disabled={save.isPending || !isComplete}>
          Save
        </Button>
      </div>
    </form>
  )
}

type NotAppliedAlertProps = {
  fileName: string
  version: string
  target: ApplyTarget
}

function NotAppliedAlert({ fileName, version, target }: NotAppliedAlertProps) {
  const [isConfirmOpen, setIsConfirmOpen] = useState(false)
  const queryClient = useQueryClient()
  const apply = useMutation({
    mutationFn: () => applyEnvFile(fileName, version),
    onSuccess: (applied) => {
      queryClient.setQueryData(['env', 'apply-target', fileName], applied)
      toast.success(`${fileName} applied.`)
    },
    // A 409 may mean the file changed meanwhile; show its newest version behind the dialog.
    onError: () => queryClient.invalidateQueries({ queryKey: ['env', 'file'] }),
  })
  const recreated =
    target.services.length > 0 ? target.services.join(', ') : 'every service'
  const confirmText =
    target.adapter === 'doco-cd'
      ? `Doco-CD recreates ${recreated} in ${target.project}. They restart with the saved values.`
      : `Docu-UI notifies the ${describeAdapter(target)} to apply ${describeServices(target)}. What happens next is up to the receiver.`

  return (
    <Alert>
      <TriangleAlert />
      <AlertTitle>Saved, not applied yet</AlertTitle>
      <AlertDescription>
        <p>The running containers still use the previous values.</p>
        <Button
          size='sm'
          className='mt-2'
          onClick={() => setIsConfirmOpen(true)}
        >
          <Rocket />
          Apply
        </Button>
      </AlertDescription>
      <ConfirmDialog
        open={isConfirmOpen}
        onOpenChange={setIsConfirmOpen}
        title={`Apply ${fileName}?`}
        desc={confirmText}
        confirmText='Apply'
        isLoading={apply.isPending}
        handleConfirm={() => apply.mutate()}
      >
        {apply.isError && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {apply.error.message}
          </p>
        )}
      </ConfirmDialog>
    </Alert>
  )
}

const NO_OWN_WEBHOOK: WebhookInput = {
  url: '',
  secret: '',
  headerName: '',
  headerValue: '',
}

function describeAdapter(target: ApplyTarget): string {
  if (target.adapter === 'doco-cd') {
    return 'Doco-CD'
  }
  return target.webhook.url === '' ? 'shared webhook' : 'own webhook'
}

function describeServices(target: ApplyTarget): string {
  const services =
    target.services.length > 0 ? target.services.join(', ') : 'whole project'
  return target.project === '' ? services : `${target.project}: ${services}`
}
