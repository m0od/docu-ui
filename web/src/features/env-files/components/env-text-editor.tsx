import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ApiError } from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { readEnvContent, saveEnvContent } from '../api/env-files-api'
import { LineDiff } from './line-diff'

type EnvTextEditorProps = {
  fileName: string
  onClose: () => void
}

// The whole file as text, every value in clear, for edits too big for one variable at a time.
export function EnvTextEditor({ fileName, onClose }: EnvTextEditorProps) {
  const fileContent = useQuery({
    queryKey: ['env', 'content', fileName],
    queryFn: () => readEnvContent(fileName),
    // Never reuse secrets from an earlier visit; always read the file as it is now.
    gcTime: 0,
  })

  if (fileContent.isPending) {
    return <Skeleton className='h-60 w-full' />
  }
  if (fileContent.isError) {
    return (
      <p role='alert' className='text-sm font-medium text-destructive'>
        {fileContent.error.message}
      </p>
    )
  }
  return (
    <TextEditorForm
      // A reload after a conflict starts over from the new content.
      key={fileContent.data.version}
      fileName={fileName}
      original={fileContent.data.content}
      version={fileContent.data.version}
      onClose={onClose}
      onReload={() => fileContent.refetch()}
    />
  )
}

type TextEditorFormProps = {
  fileName: string
  original: string
  version: string
  onClose: () => void
  onReload: () => void
}

function TextEditorForm({
  fileName,
  original,
  version,
  onClose,
  onReload,
}: TextEditorFormProps) {
  const [draft, setDraft] = useState(original)
  const [isReviewOpen, setIsReviewOpen] = useState(false)
  const queryClient = useQueryClient()
  const saveContent = useMutation({
    mutationFn: () => saveEnvContent(fileName, version, draft),
    onSuccess: async () => {
      toast.success(`${fileName} saved.`)
      await queryClient.invalidateQueries({ queryKey: ['env', 'file'] })
      onClose()
    },
  })
  const isConflict =
    saveContent.error instanceof ApiError && saveContent.error.status === 409

  return (
    <div className='grid gap-3'>
      <Textarea
        aria-label={`Content of ${fileName}`}
        className='min-h-80 font-mono text-sm'
        spellCheck={false}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
      />
      <div className='flex justify-end gap-2'>
        <Button variant='outline' onClick={onClose}>
          Cancel
        </Button>
        <Button
          disabled={draft === original}
          onClick={() => setIsReviewOpen(true)}
        >
          Review and save
        </Button>
      </div>

      <ConfirmDialog
        open={isReviewOpen}
        onOpenChange={setIsReviewOpen}
        title={`Save ${fileName}?`}
        desc='The current file is kept in its history.'
        confirmText='Save'
        className='sm:max-w-2xl'
        isLoading={saveContent.isPending}
        handleConfirm={() => saveContent.mutate()}
      >
        <LineDiff before={original} after={draft} />
        {saveContent.isError && (
          <div role='alert' className='text-sm font-medium text-destructive'>
            {saveContent.error.message}
            {isConflict && (
              <span className='block'>
                Reloading replaces your edits with the saved file; copy them
                first if needed.
                <Button
                  variant='link'
                  className='h-auto p-0 ps-2'
                  onClick={onReload}
                >
                  Reload file
                </Button>
              </span>
            )}
          </div>
        )}
      </ConfirmDialog>
    </div>
  )
}
