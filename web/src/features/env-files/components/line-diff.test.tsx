import { describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { LineDiff } from './line-diff'

describe('LineDiff', () => {
  it('marks removed and added lines, keeps unchanged ones', async () => {
    const screen = await render(
      <LineDiff before={'A=1\nB=2\n'} after={'A=1\nB=3\nC=4\n'} />
    )

    await expect.element(screen.getByText('A=1')).toBeInTheDocument()
    await expect.element(screen.getByText(/[+−] A=1/)).not.toBeInTheDocument()
    await expect.element(screen.getByText('B=2')).toHaveTextContent('− B=2')
    await expect.element(screen.getByText('B=3')).toHaveTextContent('+ B=3')
    await expect.element(screen.getByText('C=4')).toHaveTextContent('+ C=4')
  })
})
