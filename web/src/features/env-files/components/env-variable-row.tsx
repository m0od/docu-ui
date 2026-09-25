import { useState } from 'react'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { type EnvVariable, revealEnvValue } from '../api/env-files-api'

type EnvVariableRowProps = {
  fileName: string
  variable: EnvVariable
}

// Values stay masked until asked for, so a screen share or a glance over the
// shoulder does not leak every secret in the file.
export function EnvVariableRow({ fileName, variable }: EnvVariableRowProps) {
  const [revealedValue, setRevealedValue] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  async function toggleValue() {
    if (revealedValue !== null) {
      // Dropped from memory, not just hidden: hiding means it is gone from the page.
      setRevealedValue(null)
      return
    }
    setIsLoading(true)
    try {
      setRevealedValue(await revealEnvValue(fileName, variable.key))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Cannot load value.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <TableRow className={variable.overridden ? 'opacity-60' : undefined}>
      <TableCell className='w-12 text-muted-foreground tabular-nums'>
        {variable.line}
      </TableCell>
      <TableCell className='font-mono'>
        {variable.key}
        {variable.overridden && (
          <Badge variant='outline' className='ms-2'>
            overridden below
          </Badge>
        )}
      </TableCell>
      <TableCell className='font-mono break-all whitespace-normal'>
        {revealedValue ?? '••••••••'}
      </TableCell>
      <TableCell className='w-12'>
        {!variable.overridden && (
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
      </TableCell>
    </TableRow>
  )
}
