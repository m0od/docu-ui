import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { SettingsAccount } from './index'

vi.mock('./components/password-form', () => ({
  PasswordForm: () => <p>password form</p>,
}))
vi.mock('./components/turn-on-sign-in-form', () => ({
  TurnOnSignInForm: () => <p>turn on form</p>,
}))
vi.mock('./components/turn-off-sign-in-section', () => ({
  TurnOffSignInSection: (props: { totpEnabled: boolean }) => (
    <p>{`turn off ${props.totpEnabled}`}</p>
  ),
}))
vi.mock('./components/recovery-codes-section', () => ({
  RecoveryCodesSection: (props: { recoveryCodesLeft: number }) => (
    <p>{`recovery codes ${props.recoveryCodesLeft}`}</p>
  ),
}))
vi.mock('./components/two-factor-section', () => ({
  TwoFactorSection: (props: { username: string; totpEnabled: boolean }) => (
    <p>{`two-factor ${props.username} ${props.totpEnabled}`}</p>
  ),
}))

describe('SettingsAccount', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // With sign-in off there is no password or TOTP to change: only the way to turn sign-in on.
  it('offers only turning sign-in on while it is off', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ signIn: false })
    )
    const screen = await render(withQueryClient(<SettingsAccount />))

    await expect.element(screen.getByText('turn on form')).toBeInTheDocument()
    await expect
      .element(screen.getByText('password form'))
      .not.toBeInTheDocument()
  })

  it('shows the password form, the TOTP state and turning sign-in off', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({
        signIn: true,
        username: 'admin',
        totpEnabled: true,
        recoveryCodesLeft: 7,
      })
    )
    const screen = await render(withQueryClient(<SettingsAccount />))

    await expect.element(screen.getByText('password form')).toBeInTheDocument()
    await expect
      .element(screen.getByText('two-factor admin true'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('recovery codes 7'))
      .toBeInTheDocument()
    await expect.element(screen.getByText('turn off true')).toBeInTheDocument()
    await expect
      .element(screen.getByText('turn on form'))
      .not.toBeInTheDocument()
  })

  // Without TOTP there is nothing for recovery codes to stand in for.
  it('hides recovery codes while TOTP is off', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ signIn: true, username: 'admin', totpEnabled: false })
    )
    const screen = await render(withQueryClient(<SettingsAccount />))

    await expect
      .element(screen.getByText('two-factor admin false'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText(/recovery codes/))
      .not.toBeInTheDocument()
  })

  it('shows why the account cannot be read', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ error: 'cannot read accounts' }, { status: 500 })
    )
    const screen = await render(withQueryClient(<SettingsAccount />))

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('cannot read accounts')
  })
})
