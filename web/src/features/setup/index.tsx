import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '@/features/auth/auth-layout'
import { SetupForm } from './components/setup-form'

export function Setup() {
  return (
    <AuthLayout>
      <Card className='w-full max-w-md gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>
            Welcome to Docu-UI
          </CardTitle>
          <CardDescription>
            No account exists yet. Create the admin account to finish setup.
            This page closes once the account is created.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <SetupForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
