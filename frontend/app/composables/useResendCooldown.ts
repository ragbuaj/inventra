export function useResendCooldown(baseSeconds = 30) {
  const remaining = ref(0)
  const attempts = ref(0)
  const canResend = computed(() => remaining.value <= 0)
  let timer: ReturnType<typeof setInterval> | null = null

  function clear() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  function start() {
    attempts.value += 1
    remaining.value = baseSeconds * 2 ** (attempts.value - 1)
    clear()
    timer = setInterval(() => {
      remaining.value -= 1
      if (remaining.value <= 0) {
        remaining.value = 0
        clear()
      }
    }, 1000)
  }

  function reset() {
    attempts.value = 0
    remaining.value = 0
    clear()
  }

  if (typeof getCurrentScope === 'function' && getCurrentScope()) onScopeDispose(clear)
  return { remaining, attempts, canResend, start, reset }
}
