import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Undo2 } from 'lucide-react'
import { toast } from 'sonner'
import { ApiError } from '@/lib/api-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  changeEnvVariables,
  type EnvChange,
  type EnvVariable,
} from '../api/env-files-api'
import { AddVariableForm } from './add-variable-form'
import { EnvVariableRow } from './env-variable-row'

type EnvVariablesEditorProps = {
  fileName: string
  version: string
  variables: EnvVariable[]
}

// Changes are collected here and sent in one save, after a review.
export function EnvVariablesEditor({
  fileName,
  version,
  variables,
}: EnvVariablesEditorProps) {
  // key → new value, or null for "remove". Insertion order = order sent to the server.
  const [pendingChanges, setPendingChanges] = useState(
    new Map<string, string | null>()
  )
  const [isReviewOpen, setIsReviewOpen] = useState(false)
  const queryClient = useQueryClient()
  const existingKeys = new Set(variables.map((variable) => variable.key))
  const addedKeys = [...pendingChanges.keys()].filter(
    (key) => !existingKeys.has(key)
  )

  function setChange(key: string, value: string | null) {
    setPendingChanges((changes) => new Map(changes).set(key, value))
  }

  function undoChange(key: string) {
    setPendingChanges((changes) => {
      const remainingChanges = new Map(changes)
      remainingChanges.delete(key)
      return remainingChanges
    })
  }

  const saveChanges = useMutation({
    mutationFn: (changes: EnvChange[]) =>
      changeEnvVariables(fileName, version, changes),
    onSuccess: () => {
      toast.success(`${fileName} saved.`)
      setPendingChanges(new Map())
      setIsReviewOpen(false)
      return queryClient.invalidateQueries({ queryKey: ['env', 'file'] })
    },
  })
  // Someone saved the file meanwhile: reloading gives the new version, and the
  // unsaved changes stay so they can be reviewed again on top of it.
  const isConflict =
    saveChanges.error instanceof ApiError && saveChanges.error.status === 409

  return (
    <div className='grid gap-4'>
      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Line</TableHead>
              <TableHead>Key</TableHead>
              <TableHead>Value</TableHead>
              <TableHead>
                <span className='sr-only'>Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {variables.map((variable) => (
              <EnvVariableRow
                key={variable.line}
                fileName={fileName}
                variable={variable}
                pendingValue={
                  variable.overridden
                    ? undefined
                    : pendingChanges.get(variable.key)
                }
                onChange={(value) => setChange(variable.key, value)}
                onUndo={() => undoChange(variable.key)}
              />
            ))}
            {addedKeys.map((key) => (
              <TableRow key={key}>
                <TableCell className='text-muted-foreground'>new</TableCell>
                <TableCell className='font-mono'>
                  {key}
                  <Badge className='ms-2'>added</Badge>
                </TableCell>
                <TableCell className='font-mono'>••••••••</TableCell>
                <TableCell className='text-end'>
                  <Button
                    variant='ghost'
                    size='icon'
                    aria-label={`Undo ${key}`}
                    onClick={() => undoChange(key)}
                  >
                    <Undo2 />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <AddVariableForm onAdd={setChange} />

      {pendingChanges.size > 0 && (
        <div className='flex items-center justify-between gap-2 rounded-md border bg-muted/50 px-4 py-3'>
          <span className='text-sm'>
            {pendingChanges.size} unsaved change
            {pendingChanges.size === 1 ? '' : 's'}
          </span>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setPendingChanges(new Map())}
            >
              Discard
            </Button>
            <Button onClick={() => setIsReviewOpen(true)}>
              Review and save
            </Button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={isReviewOpen}
        onOpenChange={setIsReviewOpen}
        title={`Save ${fileName}?`}
        desc='Values stay hidden here. The current file is kept in its history.'
        confirmText='Save'
        isLoading={saveChanges.isPending}
        handleConfirm={() =>
          saveChanges.mutate(
            [...pendingChanges].map(([key, value]) => ({ key, value }))
          )
        }
      >
        <ul className='grid gap-1 font-mono text-sm'>
          {[...pendingChanges].map(([key, value]) => (
            <li key={key}>
              {value === null
                ? `− ${key} (removed)`
                : existingKeys.has(key)
                  ? `~ ${key} (changed)`
                  : `+ ${key} (added)`}
            </li>
          ))}
        </ul>
        {saveChanges.isError && (
          <div role='alert' className='text-sm font-medium text-destructive'>
            {saveChanges.error.message}
            {isConflict && (
              <Button
                variant='link'
                className='h-auto p-0 ps-2'
                onClick={() => {
                  saveChanges.reset()
                  return queryClient.invalidateQueries({
                    queryKey: ['env', 'file'],
                  })
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
