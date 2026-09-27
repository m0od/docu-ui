import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { RecoveryCodesPanel } from './recovery-codes-panel'

const recoveryCodes = ['abcde-fghij', 'klmno-pqrst']

describe('RecoveryCodesPanel', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists the codes and closes only when the admin says they are saved', async () => {
    const onDone = vi.fn()
    const screen = await render(
      <RecoveryCodesPanel recoveryCodes={recoveryCodes} onDone={onDone} />
    )
    await expect
      .element(screen.getByRole('listitem').nth(1))
      .toHaveTextContent('klmno-pqrst')
    expect(onDone).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: 'I saved them' }))
    expect(onDone).toHaveBeenCalledOnce()
  })

  // One code per line, so a pasted or downloaded list is easy to read and to type from.
  it('copies the codes one per line', async () => {
    const writeTextSpy = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue()
    const screen = await render(
      <RecoveryCodesPanel recoveryCodes={recoveryCodes} onDone={() => {}} />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))

    expect(writeTextSpy).toHaveBeenCalledWith('abcde-fghij\nklmno-pqrst\n')
    await expect
      .element(screen.getByRole('button', { name: 'Copied' }))
      .toBeInTheDocument()
  })

  it('downloads the codes as a text file', async () => {
    const createUrlSpy = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:codes')
    const revokeUrlSpy = vi.spyOn(URL, 'revokeObjectURL')
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => {})
    const screen = await render(
      <RecoveryCodesPanel recoveryCodes={recoveryCodes} onDone={() => {}} />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))

    const downloadedFile = createUrlSpy.mock.calls[0][0] as Blob
    await expect(downloadedFile.text()).resolves.toBe(
      'abcde-fghij\nklmno-pqrst\n'
    )
    const downloadLink = clickSpy.mock.contexts[0] as HTMLAnchorElement
    expect(downloadLink.download).toBe('docu-ui-recovery-codes.txt')
    expect(downloadLink.href).toBe('blob:codes')
    expect(revokeUrlSpy).toHaveBeenCalledWith('blob:codes')
  })
})
