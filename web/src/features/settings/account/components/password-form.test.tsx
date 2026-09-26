import { withQueryClient } from '@/test-utils/query-client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { PasswordForm } from './password-form'

function mockFetch(response: Response) {
  return vi
    .spyOn(globalThis, 'fetch')
    .mockReturnValueOnce(Promise.resolve(response))
}

async function fillPasswords(
  screen: RenderResult,
  newPassword: string,
  confirmPassword: string
) {
  await userEvent.fill(
    screen.getByLabelText('Current password'),
    'old-password'
  )
  await userEvent.fill(
    screen.getByLabelText('New password', { exact: true }),
    newPassword
  )
  await userEvent.fill(
    screen.getByLabelText('Confirm new password'),
    confirmPassword
  )
  await userEvent.click(screen.getByRole('button', { name: 'Change password' }))
}

describe('PasswordForm', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  // The server signs out every other session; the admin should know that happened.
  it('changes the password and clears the fields', async () => {
    const fetchSpy = mockFetch(new Response(null, { status: 204 }))
    const screen = await render(withQueryClient(<PasswordForm />))
    await expect
      .element(screen.getByRole('button', { name: 'Change password' }))
      .toBeDisabled()
    await fillPasswords(screen, 'a-new-long-password', 'a-new-long-password')

    await expect
      .element(
        screen.getByText('Password changed. Other sessions are signed out.')
      )
      .toBeInTheDocument()
    expect(fetchSpy).toHaveBeenCalledWith(
      'api/account/password',
      expect.objectContaining({
        body: '{"currentPassword":"old-password","newPassword":"a-new-long-password"}',
      })
    )
    await expect
      .element(screen.getByLabelText('Current password'))
      .toHaveValue('')
    await expect
      .element(screen.getByLabelText('New password', { exact: true }))
      .toHaveValue('')
  })

  // A typo in the new password would lock the admin out, so it is typed twice.
  it.each([
    [
      'a short password',
      'short',
      'short',
      'New password must be 12-256 characters.',
    ],
    [
      'a too long password',
      'x'.repeat(257),
      'x'.repeat(257),
      'New password must be 12-256 characters.',
    ],
    [
      'a mistyped confirmation',
      'a-new-long-password',
      'a-new-long-passwor',
      "Passwords don't match.",
    ],
  ])(
    'rejects %s before calling the server',
    async (_case, newPassword, confirmPassword, problem) => {
      const fetchSpy = vi.spyOn(globalThis, 'fetch')
      const screen = await render(withQueryClient(<PasswordForm />))
      await fillPasswords(screen, newPassword, confirmPassword)

      await expect.element(screen.getByRole('alert')).toHaveTextContent(problem)
      expect(fetchSpy).not.toHaveBeenCalledWith(
        'api/account/password',
        expect.anything()
      )
    }
  )

  it('shows why the server refused', async () => {
    mockFetch(
      Response.json({ error: 'current password is wrong' }, { status: 403 })
    )
    const screen = await render(withQueryClient(<PasswordForm />))
    await fillPasswords(screen, 'a-new-long-password', 'a-new-long-password')

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('current password is wrong')
  })

  // No double submit while the server is still answering.
  it('disables the button while saving', async () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(new Promise(() => {}))
    const screen = await render(withQueryClient(<PasswordForm />))
    await fillPasswords(screen, 'a-new-long-password', 'a-new-long-password')

    await expect
      .element(screen.getByRole('button', { name: 'Change password' }))
      .toBeDisabled()
  })
})
