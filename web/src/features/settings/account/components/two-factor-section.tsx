import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, ShieldCheck, ShieldOff } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { otpauthUri } from '@/features/auth/setup/api/setup-api'
import {
  disableTotp,
  enableTotp,
  fetchAccountTotpSecret,
} from '../api/account-api'

type TwoFactorSectionProps = {
  username: string
  totpEnabled: boolean
}

// Turning TOTP on or off asks for the password and a code, like signing in,
// so a session left open on another screen cannot change it.
export function TwoFactorSection({
  username,
  totpEnabled,
}: TwoFactorSectionProps) {
  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const queryClient = useQueryClient()
  const newSecret = useMutation({ mutationFn: fetchAccountTotpSecret })
  const change = useMutation({
    mutationFn: () =>
      totpEnabled
        ? disableTotp({ currentPassword, code })
        : enableTotp({ currentPassword, secret: newSecret.data ?? '', code }),
    onSuccess: () => {
      setCurrentPassword('')
      setCode('')
      newSecret.reset()
      return queryClient.invalidateQueries({ queryKey: ['account'] })
    },
  })
  const showsForm = totpEnabled || newSecret.isSuccess

  return (
    <div className='grid gap-2'>
      <div className='flex items-center gap-2'>
        <h4 className='font-medium'>Two-factor authentication</h4>
        <Badge variant={totpEnabled ? 'default' : 'secondary'}>
          {totpEnabled ? 'On' : 'Off'}
        </Badge>
      </div>
      <p className='text-sm text-muted-foreground'>
        {totpEnabled
          ? 'Sign-in asks for a code from your authenticator app.'
          : 'Optional. Ask for a code from an authenticator app at sign-in.'}
      </p>

      {!showsForm && (
        <Button
          variant='outline'
          className='justify-self-start'
          disabled={newSecret.isPending}
          onClick={() => newSecret.mutate()}
        >
          {newSecret.isPending ? (
            <Loader2 className='animate-spin' />
          ) : (
            <ShieldCheck />
          )}
          Set up
        </Button>
      )}
      {newSecret.isError && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {newSecret.error.message}
        </p>
      )}

      {showsForm && (
        <form
          className='grid gap-2'
          onSubmit={(event) => {
            event.preventDefault()
            change.mutate()
          }}
        >
          {newSecret.data && (
            <div className='flex flex-col items-center gap-3'>
              <div className='rounded-md bg-white p-3'>
                <QRCodeSVG
                  value={otpauthUri(username, newSecret.data)}
                  size={168}
                  title='TOTP QR code'
                />
              </div>
              <p className='text-center text-xs text-muted-foreground'>
                Scan with Google Authenticator, 1Password or Authy, or type this
                key:
                <code className='mt-1 block font-mono text-sm tracking-wider break-all text-foreground select-all'>
                  {newSecret.data}
                </code>
              </p>
            </div>
          )}
          <Label htmlFor='totp-current-password'>Current password</Label>
          <PasswordInput
            id='totp-current-password'
            autoComplete='current-password'
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
          />
          <Label htmlFor='totp-code'>Code from the app</Label>
          <InputOTP
            id='totp-code'
            maxLength={6}
            value={code}
            onChange={setCode}
          >
            <InputOTPGroup>
              <InputOTPSlot index={0} />
              <InputOTPSlot index={1} />
              <InputOTPSlot index={2} />
            </InputOTPGroup>
            <InputOTPSeparator />
            <InputOTPGroup>
              <InputOTPSlot index={3} />
              <InputOTPSlot index={4} />
              <InputOTPSlot index={5} />
            </InputOTPGroup>
          </InputOTP>
          <div className='flex gap-2'>
            <Button
              type='submit'
              variant={totpEnabled ? 'destructive' : 'default'}
              disabled={
                change.isPending || currentPassword === '' || code.length !== 6
              }
            >
              {change.isPending ? (
                <Loader2 className='animate-spin' />
              ) : totpEnabled ? (
                <ShieldOff />
              ) : (
                <ShieldCheck />
              )}
              {totpEnabled ? 'Turn off' : 'Turn on'}
            </Button>
            {!totpEnabled && (
              <Button
                type='button'
                variant='ghost'
                onClick={() => {
                  newSecret.reset()
                  change.reset()
                }}
              >
                Cancel
              </Button>
            )}
          </div>
          {change.isError && (
            <p role='alert' className='text-sm font-medium text-destructive'>
              {change.error.message}
            </p>
          )}
        </form>
      )}
    </div>
  )
}
