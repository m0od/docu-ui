import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ApiError } from '@/lib/api-client'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  listEnvHistory,
  readEnvContent,
  readEnvHistory,
  restoreEnvHistory,
} from '../api/env-files-api'
import { LineDiff } from './line-diff'

type EnvHistoryProps = {
  fileName: string
  onClose: () => void
}

// The versions kept on each save; pick one to compare with the file now and restore it.
export function EnvHistory({ fileName, onClose }: EnvHistoryProps) {
  const [selectedEntryId, setSelectedEntryId] = useState<string | null>(null)
  const history = useQuery({
    queryKey: ['env', 'history', fileName],
    queryFn: () => listEnvHistory(fileName),
  })

  return (
    <div className='grid gap-3'>
      <div className='flex justify-end'>
        <Button variant='outline' onClick={onClose}>
          Back to variables
        </Button>
      </div>
      {history.isPending && <Skeleton className='h-40 w-full' />}
      {history.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {history.error.message}
        </p>
      )}
      {history.isSuccess && history.data.length === 0 && (
        <p className='text-sm text-muted-foreground'>
          No versions yet. A copy is kept each time the file is saved here.
        </p>
      )}
      {history.isSuccess && history.data.length > 0 && (
        <div className='grid gap-4 md:grid-cols-[16rem_1fr]'>
          <ul className='grid content-start gap-1'>
            {history.data.map((entry) => (
              <li key={entry.id}>
                <button
                  type='button'
                  aria-pressed={entry.id === selectedEntryId}
                  className={cn(
                    'w-full rounded-md border px-3 py-2 text-start text-sm hover:bg-accent',
                    entry.id === selectedEntryId && 'bg-accent'
                  )}
                  onClick={() => setSelectedEntryId(entry.id)}
                >
                  <span className='block font-medium'>
                    {new Date(entry.savedAt).toLocaleString()}
                  </span>
                  {/* The entry is the file as it was before this save. */}
                  <span className='block text-muted-foreground'>
                    before {entry.savedBy} saved
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {selectedEntryId === null ? (
            <p className='text-sm text-muted-foreground'>
              Pick a version to compare it with the current file.
            </p>
          ) : (
            <HistoryComparison
              key={selectedEntryId}
              fileName={fileName}
              entryId={selectedEntryId}
              onRestored={onClose}
            />
          )}
        </div>
      )}
    </div>
  )
}

type HistoryComparisonProps = {
  fileName: string
  entryId: string
  onRestored: () => void
}

function HistoryComparison({
  fileName,
  entryId,
  onRestored,
}: HistoryComparisonProps) {
  const [isRestoreOpen, setIsRestoreOpen] = useState(false)
  const queryClient = useQueryClient()
  // Never reuse secrets from an earlier visit.
  const oldContent = useQuery({
    queryKey: ['env', 'history', fileName, entryId],
    queryFn: () => readEnvHistory(fileName, entryId),
    gcTime: 0,
  })
  const current = useQuery({
    queryKey: ['env', 'content', fileName],
    queryFn: () => readEnvContent(fileName),
    gcTime: 0,
  })
  const restore = useMutation({
    mutationFn: (baseVersion: string) =>
      restoreEnvHistory(fileName, entryId, baseVersion),
    onSuccess: async () => {
      toast.success(`${fileName} restored.`)
      // Only the file view: refetching the old version or the content here
      // would log views nobody made.
      await queryClient.invalidateQueries({ queryKey: ['env', 'file'] })
      onRestored()
    },
  })
  const isConflict =
    restore.error instanceof ApiError && restore.error.status === 409

  if (oldContent.isError || current.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {(oldContent.error ?? current.error)?.message}
      </p>
    )
  }
  if (!oldContent.isSuccess || !current.isSuccess) {
    return <Skeleton className='h-40 w-full' />
  }
  const isSameAsCurrent = oldContent.data === current.data.content
  return (
    <div className='grid content-start gap-3'>
      <p className='text-sm text-muted-foreground'>
        {isSameAsCurrent
          ? 'This version is the same as the current file.'
          : 'What restoring this version changes in the current file:'}
      </p>
      <LineDiff before={current.data.content} after={oldContent.data} />
      <div className='flex justify-end'>
        <Button
          disabled={isSameAsCurrent}
          onClick={() => setIsRestoreOpen(true)}
        >
          Restore this version
        </Button>
      </div>

      <ConfirmDialog
        open={isRestoreOpen}
        onOpenChange={setIsRestoreOpen}
        title={`Restore ${fileName}?`}
        desc='The current file is kept in its history, so this can be undone.'
        confirmText='Restore'
        isLoading={restore.isPending}
        handleConfirm={() => restore.mutate(current.data.version)}
      >
        {restore.isError && (
          <div role='alert' className='text-sm font-medium text-destructive'>
            {restore.error.message}
            {isConflict && (
              <Button
                variant='link'
                className='h-auto p-0 ps-2'
                onClick={() => {
                  restore.reset()
                  setIsRestoreOpen(false)
                  void current.refetch()
                }}
              >
                Reload file
              </Button>
            )}
          </div>
        )}
      </ConfirmDialog>
    </div>
  )
}
