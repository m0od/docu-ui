import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Loader2 } from 'lucide-react'
import { isCompleteCode } from '@/lib/second-factor-code'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { RecoveryCodesPanel } from '@/components/recovery-codes-panel'
import { TotpCodeInput } from '@/components/totp-code-input'
import { regenerateRecoveryCodes } from '../api/account-api'

type RecoveryCodesSectionProps = {
  recoveryCodesLeft: number
}

// At this many codes or fewer, the admin is told to create a new set before running out.
const fewRecoveryCodesLeft = 3

// New codes need both factors: whoever holds an open session must not get a way around TOTP.
export function RecoveryCodesSection({
  recoveryCodesLeft,
}: RecoveryCodesSectionProps) {
  const [showsForm, setShowsForm] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const queryClient = useQueryClient()
  const regenerate = useMutation({
    mutationFn: () => regenerateRecoveryCodes({ currentPassword, code }),
    onSuccess: (newRecoveryCodes) => {
      setRecoveryCodes(newRecoveryCodes)
      setShowsForm(false)
      setCurrentPassword('')
      setCode('')
      return queryClient.invalidateQueries({ queryKey: ['account'] })
    },
  })

  return (
    <div className='grid gap-2'>
      <h4 className='font-medium'>Recovery codes</h4>
      <p className='text-sm text-muted-foreground'>
        {recoveryCodesLeft} left. Each works once in place of a code from the
        app, if you lose your phone.
      </p>
      {recoveryCodesLeft <= fewRecoveryCodesLeft && (
        <p className='text-sm font-medium text-destructive'>
          Few codes left: create new ones.
        </p>
      )}

      {recoveryCodes.length > 0 && (
        <RecoveryCodesPanel
          recoveryCodes={recoveryCodes}
          onDone={() => setRecoveryCodes([])}
        />
      )}

      {!showsForm && recoveryCodes.length === 0 && (
        <Button
          variant='outline'
          className='justify-self-start'
          onClick={() => setShowsForm(true)}
        >
          <KeyRound />
          Create new codes
        </Button>
      )}

      {showsForm && (
        <form
          className='grid gap-2'
          onSubmit={(event) => {
            event.preventDefault()
            regenerate.mutate()
          }}
        >
          <p className='text-sm text-muted-foreground'>
            The current codes stop working at once.
          </p>
          <Label htmlFor='recovery-current-password'>Current password</Label>
          <PasswordInput
            id='recovery-current-password'
            autoComplete='current-password'
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
          />
          <Label htmlFor='recovery-code'>Code from the app</Label>
          <TotpCodeInput
            id='recovery-code'
            value={code}
            onChange={setCode}
            acceptsRecoveryCode
          />
          <div className='flex gap-2'>
            <Button
              type='submit'
              disabled={
                regenerate.isPending ||
                currentPassword === '' ||
                !isCompleteCode(code)
              }
            >
              {regenerate.isPending ? (
                <Loader2 className='animate-spin' />
              ) : (
                <KeyRound />
              )}
              Create new codes
            </Button>
            <Button
              type='button'
              variant='ghost'
              onClick={() => {
                setShowsForm(false)
                regenerate.reset()
              }}
            >
              Cancel
            </Button>
          </div>
          {regenerate.isError && (
            <p role='alert' className='text-sm font-medium text-destructive'>
              {regenerate.error.message}
            </p>
          )}
        </form>
      )}
    </div>
  )
}
