import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
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
            No account exists yet. Create the admin account, or run without
            sign-in on a personal machine. This page closes once setup is done.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <SetupForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
