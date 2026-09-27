import { describe, expect, it } from 'vitest'
import { isCompleteCode } from './second-factor-code'

describe('isCompleteCode', () => {
  // Recovery codes are typed by hand from paper or a file, so case, dash and spaces are forgiven.
  it.each([
    ['six digits', '123456', true],
    ['five digits', '12345', false],
    ['six letters', 'abcdef', false],
    ['a recovery code', 'abcde-fghij', true],
    ['an upper-case recovery code with a space', 'ABCDE FGHIJ', true],
    ['a recovery code without the dash', 'abcdefghij', true],
    ['a short recovery code', 'abcde-fghi', false],
    // 0, 1, 8 and 9 are not in the base32 alphabet the codes use.
    ['a recovery code with a 1', 'abcde-fghi1', false],
  ])('%s', (_case, code, complete) => {
    expect(isCompleteCode(code)).toBe(complete)
  })
})
