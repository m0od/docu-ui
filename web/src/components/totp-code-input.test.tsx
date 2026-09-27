import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { TotpCodeInput } from './totp-code-input'

function CodeField() {
  const [code, setCode] = useState('')
  return (
    <>
      <label htmlFor='code'>Code</label>
      <TotpCodeInput
        id='code'
        value={code}
        onChange={setCode}
        acceptsRecoveryCode
      />
      <output>{code}</output>
    </>
  )
}

describe('TotpCodeInput', () => {
  // Switching clears the field, so half a TOTP code is never sent as a recovery code.
  it('switches between the app code and a recovery code', async () => {
    const screen = await render(<CodeField />)
    await userEvent.fill(screen.getByLabelText('Code'), '123')
    await expect.element(screen.getByRole('status')).toHaveTextContent('123')

    await userEvent.click(
      screen.getByRole('button', {
        name: 'Lost your phone? Use a recovery code',
      })
    )
    await expect.element(screen.getByRole('status')).toHaveTextContent('')
    await expect
      .element(screen.getByLabelText('Code'))
      .toHaveAttribute('placeholder', 'xxxxx-xxxxx')
    await userEvent.fill(screen.getByLabelText('Code'), 'abcde-fghij')
    await expect
      .element(screen.getByRole('status'))
      .toHaveTextContent('abcde-fghij')

    await userEvent.click(
      screen.getByRole('button', { name: 'Use the code from the app' })
    )
    await expect.element(screen.getByRole('status')).toHaveTextContent('')
    await expect
      .element(screen.getByLabelText('Code'))
      .not.toHaveAttribute('placeholder')
  })

  it('offers no recovery code unless asked to', async () => {
    const screen = await render(
      <TotpCodeInput id='code' value='' onChange={() => {}} />
    )
    await expect.element(screen.getByRole('button')).not.toBeInTheDocument()
  })
})
