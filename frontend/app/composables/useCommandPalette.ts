const KEY = 'inventra.search.recent'

export function useCommandPalette() {
  const isOpen = useState('cmdp-open', () => false)
  const recent = useState<string[]>('cmdp-recent', () => {
    if (import.meta.client) {
      try {
        const raw = localStorage.getItem(KEY)
        if (raw) return JSON.parse(raw) as string[]
      } catch { /* ignore */ }
    }
    return []
  })

  function open() {
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
  }

  function toggle() {
    isOpen.value = !isOpen.value
  }

  function pushRecent(q: string) {
    const term = q.trim()
    if (!term) return
    recent.value = [term, ...recent.value.filter(x => x !== term)].slice(0, 5)
    if (import.meta.client) {
      try {
        localStorage.setItem(KEY, JSON.stringify(recent.value))
      } catch { /* ignore */ }
    }
  }

  return { isOpen, open, close, toggle, recent, pushRecent }
}
