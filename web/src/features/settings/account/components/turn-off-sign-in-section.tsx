import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, ShieldOff } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { turnOffSignIn } from '../api/account-api'
import { TotpCodeInput } from './totp-code-input'

type TurnOffSignInSectionProps = {
  totpEnabled: boolean
}

// Turning sign-in off opens every env file to anyone who reaches the URL, so it
// needs the same proof as signing in: the password, and a code when TOTP is on.
export function TurnOffSignInSection({
  totpEnabled,
}: TurnOffSignInSectionProps) {
  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const queryClient = useQueryClient()
  const turnOff = useMutation({
    mutationFn: () => turnOffSignIn({ currentPassword, code }),
    onSuccess: () => {
      useAuthStore
        .getState()
        .auth.setUser({ username: 'anonymous', signIn: false })
      toast.warning('Sign-in is off.')
      return queryClient.invalidateQueries({ queryKey: ['account'] })
    },
  })

  return (
    <form
      className='grid gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        turnOff.mutate()
      }}
    >
      <h4 className='font-medium'>Turn off sign-in</h4>
      <p className='text-sm text-muted-foreground'>
        For a personal machine only. The account and every session are deleted,
        and anyone who reaches this URL can read and change every env file.
      </p>
      <Label htmlFor='turn-off-password'>Current password</Label>
      <PasswordInput
        id='turn-off-password'
        autoComplete='current-password'
        value={currentPassword}
        onChange={(event) => setCurrentPassword(event.target.value)}
      />
      {totpEnabled && (
        <>
          <Label htmlFor='turn-off-code'>Code from the app</Label>
          <TotpCodeInput id='turn-off-code' value={code} onChange={setCode} />
        </>
      )}
      <Button
        type='submit'
        variant='destructive'
        className='justify-self-start'
        disabled={
          turnOff.isPending ||
          currentPassword === '' ||
          (totpEnabled && code.length !== 6)
        }
      >
        {turnOff.isPending ? (
          <Loader2 className='animate-spin' />
        ) : (
          <ShieldOff />
        )}
        Turn off sign-in
      </Button>
      {turnOff.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {turnOff.error.message}
        </p>
      )}
    </form>
  )
}
