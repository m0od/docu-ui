import { useQuery } from '@tanstack/react-query'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { ContentSection } from '../components/content-section'
import { fetchAccount } from './api/account-api'
import { PasswordForm } from './components/password-form'
import { TwoFactorSection } from './components/two-factor-section'

export function SettingsAccount() {
  const account = useQuery({ queryKey: ['account'], queryFn: fetchAccount })

  return (
    <ContentSection
      title='Account'
      desc='Your password and two-factor authentication.'
    >
      <>
        {account.isPending && <Skeleton className='h-40 w-full' />}
        {account.isError && (
          <p role='alert' className='text-sm font-medium text-destructive'>
            {account.error.message}
          </p>
        )}
        {account.isSuccess && (
          <div className='grid gap-6'>
            <PasswordForm />
            <Separator />
            <TwoFactorSection
              username={account.data.username}
              totpEnabled={account.data.totpEnabled}
            />
          </div>
        )}
      </>
    </ContentSection>
  )
}
