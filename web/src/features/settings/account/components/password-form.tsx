import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { KeyRound, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { changePassword } from '../api/account-api'

// Same limits as the server (internal/server/setup.go).
function passwordProblem(newPassword: string, confirmPassword: string) {
  if (newPassword.length < 12 || newPassword.length > 256) {
    return 'New password must be 12-256 characters.'
  }
  if (newPassword !== confirmPassword) {
    return "Passwords don't match."
  }
  return ''
}

export function PasswordForm() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [problem, setProblem] = useState('')
  const change = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => {
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    },
  })

  return (
    <form
      className='grid gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        const foundProblem = passwordProblem(newPassword, confirmPassword)
        setProblem(foundProblem)
        if (foundProblem === '') {
          change.mutate()
        }
      }}
    >
      <h4 className='font-medium'>Password</h4>
      <Label htmlFor='current-password'>Current password</Label>
      <PasswordInput
        id='current-password'
        autoComplete='current-password'
        value={currentPassword}
        onChange={(event) => setCurrentPassword(event.target.value)}
      />
      <Label htmlFor='new-password'>New password</Label>
      <PasswordInput
        id='new-password'
        autoComplete='new-password'
        value={newPassword}
        onChange={(event) => setNewPassword(event.target.value)}
      />
      <Label htmlFor='confirm-new-password'>Confirm new password</Label>
      <PasswordInput
        id='confirm-new-password'
        autoComplete='new-password'
        value={confirmPassword}
        onChange={(event) => setConfirmPassword(event.target.value)}
      />
      <Button
        type='submit'
        className='justify-self-start'
        disabled={change.isPending || currentPassword === ''}
      >
        {change.isPending ? <Loader2 className='animate-spin' /> : <KeyRound />}
        Change password
      </Button>
      {change.isSuccess && (
        <p className='text-sm text-muted-foreground'>
          Password changed. Other sessions are signed out.
        </p>
      )}
      {(problem !== '' || change.isError) && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {problem || change.error?.message}
        </p>
      )}
    </form>
  )
}
