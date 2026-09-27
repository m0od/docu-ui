// isCompleteCode says whether a code is ready to send: 6 digits from the app, or a
// recovery code (10 letters and digits; case, dash and spaces do not matter).
export function isCompleteCode(code: string): boolean {
  return (
    /^\d{6}$/.test(code) ||
    /^[a-z2-7]{10}$/.test(code.toLowerCase().replace(/[-\s]/g, ''))
  )
}
