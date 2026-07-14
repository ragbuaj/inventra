import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useResendCooldown } from '~/composables/useResendCooldown'

describe('useResendCooldown', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('exponential backoff 30 -> 60 -> 120', () => {
    const c = useResendCooldown(30)
    expect(c.canResend.value).toBe(true)
    c.start()
    expect(c.attempts.value).toBe(1)
    expect(c.remaining.value).toBe(30)
    expect(c.canResend.value).toBe(false)
    vi.advanceTimersByTime(30000)
    expect(c.remaining.value).toBe(0)
    expect(c.canResend.value).toBe(true)
    c.start()
    expect(c.remaining.value).toBe(60)
    vi.advanceTimersByTime(60000)
    c.start()
    expect(c.remaining.value).toBe(120)
  })

  it('reset clears attempts and timer', () => {
    const c = useResendCooldown(30)
    c.start()
    c.reset()
    expect(c.attempts.value).toBe(0)
    expect(c.remaining.value).toBe(0)
    expect(c.canResend.value).toBe(true)
  })
})
