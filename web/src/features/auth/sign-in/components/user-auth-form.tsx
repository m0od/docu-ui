import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, LogIn } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { isCompleteCode } from '@/lib/second-factor-code'
import { signIn } from '@/lib/session-api'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { TotpCodeInput } from '@/components/totp-code-input'

const formSchema = z.object({
  username: z.string().trim().min(1, 'Please enter your username.'),
  password: z.string().min(1, 'Please enter your password.'),
  // Checked on submit only once the server has asked for it.
  totpCode: z.string(),
})

type SignInValues = z.infer<typeof formSchema>

// Only same-app paths: "?redirect=https://evil.example" must not send the
// user off-site after a real sign-in.
function safeRedirectPath(redirectTo: string | undefined): string {
  if (redirectTo?.startsWith('/') && !redirectTo.startsWith('//')) {
    return redirectTo
  }
  return '/'
}

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [totpRequired, setTotpRequired] = useState(false)
  const navigate = useNavigate()
  const { auth } = useAuthStore()

  const form = useForm<SignInValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { username: '', password: '', totpCode: '' },
  })

  async function onSubmit(values: SignInValues) {
    if (totpRequired && !isCompleteCode(values.totpCode)) {
      form.setError('totpCode', {
        message: 'Enter the 6-digit code or a recovery code.',
      })
      return
    }
    setIsLoading(true)
    try {
      const result = await signIn(values)
      if (result.status === 'totp-required') {
        setTotpRequired(true)
        form.setError('root', { message: result.message })
        return
      }
      auth.setUser({ username: result.username, signIn: true })
      navigate({ to: safeRedirectPath(redirectTo), replace: true })
    } catch (error) {
      // A used or wrong code cannot be retried; clear it for the next one.
      form.setValue('totpCode', '')
      form.setError('root', {
        message: error instanceof Error ? error.message : 'Sign-in failed.',
      })
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-3', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Username</FormLabel>
              <FormControl>
                <Input
                  autoComplete='username'
                  readOnly={totpRequired}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem className='relative'>
              <FormLabel>Password</FormLabel>
              <FormControl>
                <PasswordInput
                  autoComplete='current-password'
                  placeholder='********'
                  readOnly={totpRequired}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        {totpRequired && (
          <FormField
            control={form.control}
            name='totpCode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Code from your authenticator app</FormLabel>
                <FormControl>
                  <TotpCodeInput
                    value={field.value}
                    onChange={field.onChange}
                    acceptsRecoveryCode
                    autoFocus
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        )}

        {form.formState.errors.root && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {form.formState.errors.root.message}
          </p>
        )}

        <Button className='mt-2' disabled={isLoading}>
          {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
          Sign in
        </Button>
      </form>
    </Form>
  )
}
