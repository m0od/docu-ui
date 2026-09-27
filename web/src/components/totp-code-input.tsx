import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp'

type TotpCodeInputProps = {
  // Optional inside a FormControl, which passes the id its label points to.
  id?: string
  value: string
  onChange: (code: string) => void
  // Where the account's own TOTP is checked, a recovery code works too (the phone may be lost).
  acceptsRecoveryCode?: boolean
  autoFocus?: boolean
}

// The 6-digit code from the authenticator app, as 3 + 3 boxes.
export function TotpCodeInput({
  id,
  value,
  onChange,
  acceptsRecoveryCode = false,
  autoFocus = false,
}: TotpCodeInputProps) {
  const [usesRecoveryCode, setUsesRecoveryCode] = useState(false)

  return (
    <div className='grid justify-items-start gap-1'>
      {usesRecoveryCode ? (
        <Input
          id={id}
          autoFocus
          autoComplete='off'
          spellCheck={false}
          placeholder='xxxxx-xxxxx'
          className='font-mono'
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
      ) : (
        <InputOTP
          id={id}
          maxLength={6}
          autoFocus={autoFocus}
          value={value}
          onChange={onChange}
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
      )}
      {acceptsRecoveryCode && (
        <Button
          type='button'
          variant='link'
          className='h-auto p-0 text-xs'
          onClick={() => {
            setUsesRecoveryCode(!usesRecoveryCode)
            onChange('')
          }}
        >
          {usesRecoveryCode
            ? 'Use the code from the app'
            : 'Lost your phone? Use a recovery code'}
        </Button>
      )}
    </div>
  )
}
