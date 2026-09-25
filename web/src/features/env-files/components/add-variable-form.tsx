import { useState } from 'react'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// Same rule as the server: what Compose accepts as a variable name.
const KEY_PATTERN = /^[A-Za-z_][A-Za-z0-9_.-]*$/

type AddVariableFormProps = {
  onAdd: (key: string, value: string) => void
}

export function AddVariableForm({ onAdd }: AddVariableFormProps) {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const trimmedKey = key.trim()
  const keyError =
    trimmedKey !== '' && !KEY_PATTERN.test(trimmedKey)
      ? 'Use letters, digits, _ . - and start with a letter or _.'
      : null

  return (
    <form
      className='grid gap-1'
      onSubmit={(event) => {
        event.preventDefault()
        onAdd(trimmedKey, value)
        setKey('')
        setValue('')
      }}
    >
      <div className='flex gap-2'>
        <Input
          aria-label='New key'
          placeholder='NEW_KEY'
          className='max-w-60 font-mono'
          value={key}
          onChange={(event) => setKey(event.target.value)}
        />
        <Input
          aria-label='New value'
          placeholder='value'
          className='font-mono'
          value={value}
          onChange={(event) => setValue(event.target.value)}
        />
        <Button
          type='submit'
          variant='outline'
          disabled={trimmedKey === '' || keyError !== null}
        >
          <Plus />
          Add
        </Button>
      </div>
      {keyError && (
        <p className='text-sm text-destructive' role='alert'>
          {keyError}
        </p>
      )}
    </form>
  )
}
