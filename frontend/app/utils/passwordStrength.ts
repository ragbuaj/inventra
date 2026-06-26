const LABELS = ['', 'account.strength.weak', 'account.strength.fair', 'account.strength.strong', 'account.strength.veryStrong']

export function passwordStrength(pw: string): { score: 0 | 1 | 2 | 3 | 4, labelKey: string } {
  let s = 0
  if (pw.length >= 8) s++
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) s++
  if (/\d/.test(pw)) s++
  if (/[^A-Za-z0-9]/.test(pw)) s++
  const score = Math.min(s, 4) as 0 | 1 | 2 | 3 | 4
  return { score, labelKey: LABELS[score]! }
}
