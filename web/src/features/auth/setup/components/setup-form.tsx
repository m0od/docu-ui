import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, ShieldCheck, TriangleAlert } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { Switch } from '@/components/ui/switch'
import { PasswordInput } from '@/components/password-input'
import {
  createFirstAccount,
  fetchTotpSecret,
  otpauthUri,
  skipSignIn,
} from '@/features/auth/setup/api/setup-api'

// Same limits as the server (internal/server/setup.go), so most mistakes are caught before submitting.
// Skipping sign-in needs only the setup token, so the account rules apply only without the tick.
const formSchema = z
  .object({
    setupToken: z
      .string()
      .trim()
      .min(1, 'Paste the setup token from the server log.'),
    skipSignIn: z.boolean(),
    username: z.string(),
    password: z.string(),
    confirmPassword: z.string(),
    enableTotp: z.boolean(),
    totpCode: z.string(),
  })
  .superRefine((values, context) => {
    if (values.skipSignIn) {
      return
    }
    const problems: [keyof SetupFormValues, string, boolean][] = [
      [
        'username',
        'Use 3-64 letters, digits, dot, dash or underscore.',
        !/^[A-Za-z0-9._-]{3,64}$/.test(values.username),
      ],
      [
        'password',
        'Password must be at least 12 characters long.',
        values.password.length < 12,
      ],
      [
        'password',
        'Password must be at most 256 characters long.',
        values.password.length > 256,
      ],
      [
        'confirmPassword',
        'Please confirm your password.',
        values.confirmPassword === '',
      ],
      [
        'confirmPassword',
        "Passwords don't match.",
        values.confirmPassword !== '' &&
          values.password !== values.confirmPassword,
      ],
      [
        'totpCode',
        'Enter the 6-digit code from your authenticator app.',
        values.enableTotp && values.totpCode.length !== 6,
      ],
    ]
    for (const [field, message, failed] of problems) {
      if (failed) {
        context.addIssue({ code: 'custom', path: [field], message })
      }
    }
  })

type SetupFormValues = {
  setupToken: string
  skipSignIn: boolean
  username: string
  password: string
  confirmPassword: string
  enableTotp: boolean
  totpCode: string
}

export function SetupForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const navigate = useNavigate()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [totpSecret, setTotpSecret] = useState('')

  const form = useForm<SetupFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      setupToken: '',
      skipSignIn: false,
      username: 'admin',
      password: '',
      confirmPassword: '',
      enableTotp: false,
      totpCode: '',
    },
  })
  // eslint-disable-next-line react-hooks/incompatible-library
  const [enableTotp, username, skipsSignIn] = form.watch([
    'enableTotp',
    'username',
    'skipSignIn',
  ])

  async function toggleTotp(checked: boolean) {
    form.setValue('enableTotp', checked)
    form.setValue('totpCode', '')
    if (checked && totpSecret === '') {
      try {
        setTotpSecret(await fetchTotpSecret())
      } catch (error) {
        form.setValue('enableTotp', false)
        toast.error((error as Error).message)
      }
    }
  }

  async function onSubmit(values: SetupFormValues) {
    setIsSubmitting(true)
    try {
      if (values.skipSignIn) {
        await skipSignIn(values.setupToken.trim())
        toast.warning('Setup done without sign-in.')
        navigate({ to: '/', replace: true })
        return
      }
      await createFirstAccount({
        setupToken: values.setupToken.trim(),
        username: values.username,
        password: values.password,
        totpSecret: values.enableTotp ? totpSecret : '',
        totpCode: values.enableTotp ? values.totpCode : '',
      })
      toast.success(`Account ${values.username} created. Sign in to continue.`)
      navigate({ to: '/sign-in', replace: true })
    } catch (error) {
      form.setError('root', { message: (error as Error).message })
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='setupToken'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Setup token</FormLabel>
              <FormControl>
                <Input autoComplete='off' spellCheck={false} {...field} />
              </FormControl>
              <FormDescription>
                Printed in the server log:{' '}
                <code className='rounded bg-muted px-1 py-0.5 text-xs'>
                  docker logs docu-ui | grep setup_token
                </code>
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='skipSignIn'
          render={({ field }) => (
            <FormItem className='flex items-start gap-3 rounded-lg border p-4'>
              <FormControl>
                <Checkbox
                  checked={field.value}
                  onCheckedChange={(checked) =>
                    field.onChange(checked === true)
                  }
                />
              </FormControl>
              <div className='space-y-1'>
                <FormLabel>Don't set up sign-in</FormLabel>
                <FormDescription>
                  For a personal machine: open Docu-UI without signing in. You
                  can turn sign-in on later in Settings → Account.
                </FormDescription>
              </div>
            </FormItem>
          )}
        />

        {skipsSignIn ? (
          <p
            role='alert'
            className='flex gap-2 rounded-lg border border-destructive bg-destructive/10 p-3 text-sm text-destructive'
          >
            <TriangleAlert className='mt-0.5 size-4 shrink-0' />
            <span>
              <strong>No sign-in:</strong> anyone who reaches this URL can read
              and change every env file, and apply them. Only use this where
              nobody else can reach Docu-UI.
            </span>
          </p>
        ) : (
          <>
            <FormField
              control={form.control}
              name='username'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Username</FormLabel>
                  <FormControl>
                    <Input autoComplete='username' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Password</FormLabel>
                  <FormControl>
                    <PasswordInput autoComplete='new-password' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='confirmPassword'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Confirm password</FormLabel>
                  <FormControl>
                    <PasswordInput autoComplete='new-password' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='rounded-lg border p-4'>
              <div className='flex items-center justify-between gap-4'>
                <div className='space-y-1'>
                  <label htmlFor='enable-totp' className='text-sm font-medium'>
                    Two-factor authentication
                  </label>
                  <p className='text-sm text-muted-foreground'>
                    Optional. Ask for a code from an authenticator app at
                    sign-in.
                  </p>
                </div>
                <Switch
                  id='enable-totp'
                  checked={enableTotp}
                  onCheckedChange={toggleTotp}
                />
              </div>

              {enableTotp && totpSecret !== '' && (
                <div className='mt-4 grid gap-4'>
                  <div className='flex flex-col items-center gap-3'>
                    <div className='rounded-md bg-white p-3'>
                      <QRCodeSVG
                        value={otpauthUri(username, totpSecret)}
                        size={168}
                        title='TOTP QR code'
                      />
                    </div>
                    <p className='text-center text-xs text-muted-foreground'>
                      Scan with Google Authenticator, 1Password or Authy, or
                      type this key:
                      <code className='mt-1 block font-mono text-sm tracking-wider break-all text-foreground select-all'>
                        {totpSecret}
                      </code>
                    </p>
                  </div>
                  <FormField
                    control={form.control}
                    name='totpCode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Code from the app</FormLabel>
                        <FormControl>
                          <InputOTP
                            maxLength={6}
                            {...field}
                            containerClassName='justify-center'
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
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
            </div>
          </>
        )}

        {form.formState.errors.root && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {form.formState.errors.root.message}
          </p>
        )}

        <Button disabled={isSubmitting}>
          {isSubmitting ? (
            <Loader2 className='animate-spin' />
          ) : (
            <ShieldCheck />
          )}
          {skipsSignIn
            ? 'Finish setup without sign-in'
            : 'Create admin account'}
        </Button>
      </form>
    </Form>
  )
}
