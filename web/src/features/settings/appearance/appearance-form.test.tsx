import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { AppearanceForm } from './appearance-form'

const mocks = vi.hoisted(() => ({ setFont: vi.fn(), setTheme: vi.fn() }))

vi.mock('@/context/font-provider', () => ({
  useFont: () => ({ font: 'inter', setFont: mocks.setFont }),
}))
vi.mock('@/context/theme-provider', () => ({
  useTheme: () => ({ theme: 'light', setTheme: mocks.setTheme }),
}))

describe('AppearanceForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Saving applies the chosen font and theme; there is no demo toast any more.
  it('applies a new font and theme', async () => {
    const screen = await render(<AppearanceForm />)
    await userEvent.selectOptions(screen.getByLabelText('Font'), 'manrope')
    await userEvent.click(screen.getByText('Dark'))
    await userEvent.click(
      screen.getByRole('button', { name: 'Update preferences' })
    )

    await vi.waitFor(() =>
      expect(mocks.setFont).toHaveBeenCalledWith('manrope')
    )
    expect(mocks.setTheme).toHaveBeenCalledWith('dark')
  })

  // Unchanged choices are left alone, so saving does not re-trigger a theme switch.
  it('leaves unchanged choices alone', async () => {
    const screen = await render(<AppearanceForm />)
    await userEvent.click(
      screen.getByRole('button', { name: 'Update preferences' })
    )

    // Give the async form submit time to run before asserting nothing happened.
    await new Promise((resolve) => setTimeout(resolve, 200))
    expect(mocks.setFont).not.toHaveBeenCalled()
    expect(mocks.setTheme).not.toHaveBeenCalled()
  })
})
