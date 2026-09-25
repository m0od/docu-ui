import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleCheck, Rocket, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  applyEnvFile,
  fetchApplyState,
  saveApplyTarget,
  type ApplyTarget,
} from '../api/apply-api'

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
          Applies to <code className='font-mono'>{target.project}</code>
          {target.services.length > 0 ? (
            <>
              {': '}
              <code className='font-mono'>{target.services.join(', ')}</code>
            </>
          ) : (
            ' (whole project)'
          )}
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
  const [project, setProject] = useState(savedTarget?.project ?? '')
  const [services, setServices] = useState(
    savedTarget?.services.join(' ') ?? ''
  )
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: () =>
      saveApplyTarget(fileName, {
        project: project.trim(),
        services: services.split(/[\s,]+/).filter(Boolean),
      }),
    onSuccess: (saved) => {
      queryClient.setQueryData(['env', 'apply-target', fileName], saved)
      onDone()
    },
  })

  return (
    <form
      className='grid gap-2 rounded-md border p-3'
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate()
      }}
    >
      <p className='text-sm font-medium'>Where is this file used?</p>
      <div className='grid gap-2 sm:grid-cols-2'>
        <div className='grid gap-1'>
          <Label htmlFor='apply-project'>Compose project</Label>
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
        <Button
          type='submit'
          disabled={save.isPending || project.trim() === ''}
        >
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
        desc={`Doco-CD recreates ${recreated} in ${target.project}. They restart with the saved values.`}
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
