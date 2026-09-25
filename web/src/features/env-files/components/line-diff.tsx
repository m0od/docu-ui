import { diffLines } from 'diff'
import { cn } from '@/lib/utils'

type LineDiffProps = {
  before: string
  after: string
}

// LineDiff shows added lines in green with "+", removed in red with "−".
export function LineDiff({ before, after }: LineDiffProps) {
  const lines = diffLines(before, after).flatMap((part) =>
    part.value
      .replace(/\n$/, '')
      .split('\n')
      .map((text) => ({ text, added: part.added, removed: part.removed }))
  )
  return (
    <pre className='max-h-80 overflow-auto rounded-md border font-mono text-xs'>
      {lines.map((line, index) => (
        <div
          key={index}
          className={cn(
            'px-2 whitespace-pre-wrap',
            line.added && 'bg-green-500/15',
            line.removed && 'bg-red-500/15'
          )}
        >
          {line.added ? '+ ' : line.removed ? '− ' : '  '}
          {line.text}
        </div>
      ))}
    </pre>
  )
}
