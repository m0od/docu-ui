import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { AddVariableForm } from './add-variable-form'

describe('AddVariableForm', () => {
  it('adds the trimmed key with its value and clears the fields', async () => {
    const onAdd = vi.fn()
    const screen = await render(<AddVariableForm onAdd={onAdd} />)

    await userEvent.fill(screen.getByLabelText('New key'), ' KC_PORT ')
    await userEvent.fill(screen.getByLabelText('New value'), '8080')
    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(onAdd).toHaveBeenCalledWith('KC_PORT', '8080')
    await expect.element(screen.getByLabelText('New key')).toHaveValue('')
  })

  // Compose would reject the file on the next deploy.
  it('refuses a key Compose cannot read', async () => {
    const screen = await render(<AddVariableForm onAdd={vi.fn()} />)

    await expect
      .element(screen.getByRole('button', { name: 'Add' }))
      .toBeDisabled()
    await userEvent.fill(screen.getByLabelText('New key'), '1BAD KEY')

    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('start with a letter')
    await expect
      .element(screen.getByRole('button', { name: 'Add' }))
      .toBeDisabled()
  })
})
