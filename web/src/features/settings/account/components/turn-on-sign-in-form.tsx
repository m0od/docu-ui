import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { turnOnSignIn } from '../api/account-api'

// Same limits as the server (internal/server/setup.go).
function accountProblem(
  username: string,
  password: string,
  confirmPassword: string
) {
  if (!/^[A-Za-z0-9._-]{3,64}$/.test(username)) {
    return 'Username: use 3-64 letters, digits, dot, dash or underscore.'
  }
  if (password.length < 12 || password.length > 256) {
    return 'Password must be 12-256 characters.'
  }
  if (password !== confirmPassword) {
    return "Passwords don't match."
  }
  return ''
}

// Shown while sign-in is off. Creating the account locks Docu-UI at once,
// so this browser goes to sign-in next. TOTP can be turned on after signing in.
export function TurnOnSignInForm() {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [problem, setProblem] = useState('')
  const navigate = useNavigate()
  const turnOn = useMutation({
    mutationFn: () => turnOnSignIn(username, password),
    onSuccess: () => {
      useAuthStore.getState().auth.reset()
      toast.success(`Sign-in is on. Sign in as ${username}.`)
      navigate({ to: '/sign-in', replace: true })
    },
  })

  return (
    <form
      className='grid gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        const foundProblem = accountProblem(username, password, confirmPassword)
        setProblem(foundProblem)
        if (foundProblem === '') {
          turnOn.mutate()
        }
      }}
    >
      <h4 className='font-medium'>Sign-in is off</h4>
      <p className='text-sm text-muted-foreground'>
        Anyone who reaches this URL can read and change every env file. Create
        an account to require sign-in.
      </p>
      <Label htmlFor='new-username'>Username</Label>
      <Input
        id='new-username'
        autoComplete='username'
        value={username}
        onChange={(event) => setUsername(event.target.value)}
      />
      <Label htmlFor='new-account-password'>Password</Label>
      <PasswordInput
        id='new-account-password'
        autoComplete='new-password'
        value={password}
        onChange={(event) => setPassword(event.target.value)}
      />
      <Label htmlFor='confirm-account-password'>Confirm password</Label>
      <PasswordInput
        id='confirm-account-password'
        autoComplete='new-password'
        value={confirmPassword}
        onChange={(event) => setConfirmPassword(event.target.value)}
      />
      <Button
        type='submit'
        className='justify-self-start'
        disabled={turnOn.isPending}
      >
        {turnOn.isPending ? (
          <Loader2 className='animate-spin' />
        ) : (
          <ShieldCheck />
        )}
        Turn on sign-in
      </Button>
      {(problem !== '' || turnOn.isError) && (
        <p role='alert' className='text-sm font-medium text-destructive'>
          {problem || turnOn.error?.message}
        </p>
      )}
    </form>
  )
}
