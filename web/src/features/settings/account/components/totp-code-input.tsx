import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp'

type TotpCodeInputProps = {
  id: string
  value: string
  onChange: (code: string) => void
}

// The 6-digit code from the authenticator app, as 3 + 3 boxes.
export function TotpCodeInput({ id, value, onChange }: TotpCodeInputProps) {
  return (
    <InputOTP id={id} maxLength={6} value={value} onChange={onChange}>
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
  )
}
