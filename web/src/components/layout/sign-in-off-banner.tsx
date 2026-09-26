import { Link } from '@tanstack/react-router'
import { TriangleAlert } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'

// Shown on every page while sign-in is off, so nobody forgets the UI is open to anyone.
export function SignInOffBanner() {
  const signInOff = useAuthStore((state) => state.auth.user?.signIn === false)
  if (!signInOff) {
    return null
  }
  return (
    <div className='flex flex-wrap items-center gap-x-2 gap-y-1 bg-destructive px-4 py-2 text-sm text-white'>
      <TriangleAlert className='size-4 shrink-0' />
      <span>
        <strong>No sign-in:</strong> anyone who reaches this URL can read every
        value.
      </span>
      <Link to='/settings/account' className='font-medium underline'>
        Turn on sign-in
      </Link>
    </div>
  )
}
