import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { SettingsAccount } from './index'

vi.mock('./components/password-form', () => ({
  PasswordForm: () => <p>password form</p>,
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

  it('shows the password form and the TOTP state', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ username: 'admin', totpEnabled: true })
    )
    const screen = await render(withQueryClient(<SettingsAccount />))

    await expect.element(screen.getByText('password form')).toBeInTheDocument()
    await expect
      .element(screen.getByText('two-factor admin true'))
      .toBeInTheDocument()
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
