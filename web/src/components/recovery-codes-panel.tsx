import { useState } from 'react'
import { Check, Copy, Download, KeyRound } from 'lucide-react'
import { Button } from '@/components/ui/button'

type RecoveryCodesPanelProps = {
  recoveryCodes: string[]
  onDone: () => void
}

const recoveryCodesFileName = 'docu-ui-recovery-codes.txt'

// The server keeps only hashes, so this is the one time the admin can see the codes.
export function RecoveryCodesPanel({
  recoveryCodes,
  onDone,
}: RecoveryCodesPanelProps) {
  const [copied, setCopied] = useState(false)
  const codesText = recoveryCodes.join('\n') + '\n'

  async function copyCodes() {
    await navigator.clipboard.writeText(codesText)
    setCopied(true)
  }

  function downloadCodes() {
    const fileUrl = URL.createObjectURL(
      new Blob([codesText], { type: 'text/plain' })
    )
    const downloadLink = document.createElement('a')
    downloadLink.href = fileUrl
    downloadLink.download = recoveryCodesFileName
    downloadLink.click()
    URL.revokeObjectURL(fileUrl)
  }

  return (
    <section
      aria-label='Recovery codes'
      className='grid gap-3 rounded-lg border p-4'
    >
      <h4 className='flex items-center gap-2 font-medium'>
        <KeyRound className='size-4' />
        Save your recovery codes
      </h4>
      <p className='text-sm text-muted-foreground'>
        If you lose your phone, type one of these instead of the code from the
        app. Each works once. Keep them somewhere other than the phone: they are
        not shown again.
      </p>
      <ul className='grid grid-cols-2 gap-x-6 gap-y-1 rounded-md bg-muted p-3 font-mono text-sm select-all'>
        {recoveryCodes.map((recoveryCode) => (
          <li key={recoveryCode}>{recoveryCode}</li>
        ))}
      </ul>
      <div className='flex flex-wrap gap-2'>
        <Button type='button' variant='outline' onClick={copyCodes}>
          {copied ? <Check /> : <Copy />}
          {copied ? 'Copied' : 'Copy'}
        </Button>
        <Button type='button' variant='outline' onClick={downloadCodes}>
          <Download />
          Download
        </Button>
        <Button type='button' className='ml-auto' onClick={onDone}>
          I saved them
        </Button>
      </div>
    </section>
  )
}
