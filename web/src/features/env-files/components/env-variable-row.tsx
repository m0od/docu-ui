import { useState } from 'react'
import {
  Check,
  Eye,
  EyeOff,
  Loader2,
  Pencil,
  Trash2,
  Undo2,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { TableCell, TableRow } from '@/components/ui/table'
import { type EnvVariable, revealEnvValue } from '../api/env-files-api'

const MASK = '••••••••'

type EnvVariableRowProps = {
  fileName: string
  variable: EnvVariable
  // undefined: no unsaved change; null: removal; string: new value.
  pendingValue: string | null | undefined
  onChange: (value: string | null) => void
  onUndo: () => void
}

// Values stay masked until asked for, so a screen share or a glance over the
// shoulder does not leak every secret in the file.
export function EnvVariableRow({
  fileName,
  variable,
  pendingValue,
  onChange,
  onUndo,
}: EnvVariableRowProps) {
  const [revealedValue, setRevealedValue] = useState<string | null>(null)
  const [draftValue, setDraftValue] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  async function loadValue(): Promise<string | null> {
    setIsLoading(true)
    try {
      return await revealEnvValue(fileName, variable.key)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Cannot load value.')
      return null
    } finally {
      setIsLoading(false)
    }
  }

  async function toggleValue() {
    if (revealedValue !== null) {
      // Dropped from memory, not just hidden: hiding means it is gone from the page.
      setRevealedValue(null)
      return
    }
    setRevealedValue(await loadValue())
  }

  async function startEditing() {
    // Start from the unsaved value if there is one, else from the file.
    const startValue = pendingValue ?? revealedValue ?? (await loadValue())
    if (startValue === null) {
      return // could not load; loadValue already told the user
    }
    setDraftValue(startValue)
  }

  function finishEditing(value: string) {
    onChange(value)
    setDraftValue(null)
  }

  const isRemoved = pendingValue === null
  const isChanged = typeof pendingValue === 'string'
  // Actions act on the line Compose uses; an overridden duplicate is read-only.
  const canEdit = !variable.overridden

  return (
    <TableRow className={cn(variable.overridden && 'opacity-60')}>
      <TableCell className='w-12 text-muted-foreground tabular-nums'>
        {variable.line}
      </TableCell>
      <TableCell className={cn('font-mono', isRemoved && 'line-through')}>
        {variable.key}
        {variable.overridden && (
          <Badge variant='outline' className='ms-2'>
            overridden below
          </Badge>
        )}
        {isChanged && <Badge className='ms-2'>changed</Badge>}
        {isRemoved && (
          <Badge variant='destructive' className='ms-2'>
            removed
          </Badge>
        )}
      </TableCell>
      <TableCell className='font-mono break-all whitespace-normal'>
        {draftValue !== null ? (
          <form
            className='flex gap-1'
            onSubmit={(event) => {
              event.preventDefault()
              finishEditing(draftValue)
            }}
          >
            <Input
              aria-label={`New value of ${variable.key}`}
              className='font-mono'
              autoFocus
              value={draftValue}
              onChange={(event) => setDraftValue(event.target.value)}
            />
            <Button
              type='submit'
              size='icon'
              variant='ghost'
              aria-label='Apply'
            >
              <Check />
            </Button>
            <Button
              type='button'
              size='icon'
              variant='ghost'
              aria-label='Cancel edit'
              onClick={() => setDraftValue(null)}
            >
              <X />
            </Button>
          </form>
        ) : isChanged || isRemoved ? (
          MASK
        ) : (
          (revealedValue ?? MASK)
        )}
      </TableCell>
      <TableCell className='w-36 text-end whitespace-nowrap'>
        {canEdit && draftValue === null && (
          <>
            {isChanged || isRemoved ? (
              <Button
                variant='ghost'
                size='icon'
                aria-label={`Undo ${variable.key}`}
                onClick={onUndo}
              >
                <Undo2 />
              </Button>
            ) : (
              <Button
                variant='ghost'
                size='icon'
                disabled={isLoading}
                aria-label={
                  revealedValue === null
                    ? `Show ${variable.key}`
                    : `Hide ${variable.key}`
                }
                onClick={toggleValue}
              >
                {isLoading ? (
                  <Loader2 className='animate-spin' />
                ) : revealedValue === null ? (
                  <Eye />
                ) : (
                  <EyeOff />
                )}
              </Button>
            )}
            {!isRemoved && (
              <Button
                variant='ghost'
                size='icon'
                disabled={isLoading}
                aria-label={`Edit ${variable.key}`}
                onClick={startEditing}
              >
                <Pencil />
              </Button>
            )}
            {!isRemoved && (
              <Button
                variant='ghost'
                size='icon'
                aria-label={`Remove ${variable.key}`}
                onClick={() => onChange(null)}
              >
                <Trash2 />
              </Button>
            )}
          </>
        )}
      </TableCell>
    </TableRow>
  )
}
