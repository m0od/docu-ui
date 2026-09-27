import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { RecoveryCodesSection } from './recovery-codes-section'

async function openFormAndConfirm(screen: RenderResult, code: string) {
  await userEvent.click(
    screen.getByRole('button', { name: 'Create new codes' })
  )
  await userEvent.fill(screen.getByLabelText('Current password'), 'pw')
  await userEvent.fill(screen.getByLabelText('Code from the app'), code)
  await userEvent.click(
    screen.getByRole('button', { name: 'Create new codes' })
  )
}

describe('RecoveryCodesSection', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('says how many codes are left', async () => {
    const screen = await render(
      withQueryClient(<RecoveryCodesSection recoveryCodesLeft={7} />)
    )
    await expect.element(screen.getByText('7 left.')).toBeInTheDocument()
    await expect
      .element(screen.getByText('Few codes left: create new ones.'))
      .not.toBeInTheDocument()
  })

  // Running out means a lost phone locks the admin out, so they are warned early.
  it('warns when few codes are left', async () => {
    const screen = await render(
      withQueryClient(<RecoveryCodesSection recoveryCodesLeft={3} />)
    )
    await expect
      .element(screen.getByText('Few codes left: create new ones.'))
      .toBeInTheDocument()
  })

  it('creates new codes with the password and a code, and shows them once', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(Response.json({ recoveryCodes: ['abcde-fghij'] }))
      .mockResolvedValue(Response.json({}))
    const screen = await render(
      withQueryClient(<RecoveryCodesSection recoveryCodesLeft={2} />)
    )
    await openFormAndConfirm(screen, '123456')

    const recoveryPanel = screen.getByRole('region', { name: 'Recovery codes' })
    await expect.element(recoveryPanel).toHaveTextContent('abcde-fghij')
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/account/recovery-codes',
      expect.objectContaining({
        body: '{"currentPassword":"pw","code":"123456"}',
      })
    )
    await expect
      .element(screen.getByLabelText('Current password'))
      .not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'I saved them' }))
    await expect.element(recoveryPanel).not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Create new codes' }))
      .toBeInTheDocument()
  })

  it('shows why the server refused, and cancels', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      Response.json({ error: 'TOTP code is wrong' }, { status: 403 })
    )
    const screen = await render(
      withQueryClient(<RecoveryCodesSection recoveryCodesLeft={5} />)
    )
    await openFormAndConfirm(screen, '000000')
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('TOTP code is wrong')

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await expect
      .element(screen.getByLabelText('Current password'))
      .not.toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Create new codes' })
    )
    await expect.element(screen.getByRole('alert')).not.toBeInTheDocument()
  })

  // No double submit: a second set would silently replace the one on screen.
  it('disables the button while creating', async () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(new Promise(() => {}))
    const screen = await render(
      withQueryClient(<RecoveryCodesSection recoveryCodesLeft={5} />)
    )
    await openFormAndConfirm(screen, '123456')
    await expect
      .element(screen.getByRole('button', { name: 'Create new codes' }))
      .toBeDisabled()
  })
})
