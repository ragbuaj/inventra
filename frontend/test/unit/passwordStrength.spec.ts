import { describe, it, expect } from 'vitest'
import { passwordStrength } from '~/utils/passwordStrength'

describe('passwordStrength', () => {
  it('scores 0 for empty', () => {
    expect(passwordStrength('')).toEqual({ score: 0, labelKey: '' })
  })
  it('scores low for a short simple password', () => {
    expect(passwordStrength('abc').score).toBe(0)
  })
  it('rewards length, case mix, digit, and symbol', () => {
    expect(passwordStrength('abcdefgh').score).toBe(1)
    expect(passwordStrength('Abcdefgh').score).toBe(2)
    expect(passwordStrength('Abcdefg1').score).toBe(3)
    expect(passwordStrength('Abcdefg1!').score).toBe(4)
  })
  it('maps score to a label key', () => {
    expect(passwordStrength('Abcdefg1!').labelKey).toBe('account.strength.veryStrong')
    expect(passwordStrength('abcdefgh').labelKey).toBe('account.strength.weak')
  })
})
