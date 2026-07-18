/**
 * Reactive `matchMedia` wrapper. Returns a ref that tracks whether the given
 * media query currently matches, updating on viewport changes. SPA-only app
 * (`ssr: false`), so `window` is always available on the client; the SSR guard
 * keeps it safe for the Nuxt test runtime where `window` may be absent.
 */
export function useMediaQuery(query: string, defaultValue = false): Ref<boolean> {
  // In environments without `matchMedia` (SSR, the Nuxt test runtime) the ref
  // keeps `defaultValue`; real browsers overwrite it synchronously below.
  const matches = ref(defaultValue)

  if (import.meta.client && typeof window !== 'undefined' && window.matchMedia) {
    const mql = window.matchMedia(query)
    matches.value = mql.matches

    const onChange = (e: MediaQueryListEvent) => {
      matches.value = e.matches
    }
    mql.addEventListener('change', onChange)
    onScopeDispose(() => mql.removeEventListener('change', onChange))
  }

  return matches
}

/**
 * True from Tailwind's `lg` breakpoint (>= 1024px) upward. Defaults to `true`
 * (assume desktop) when `matchMedia` is unavailable so the rail-collapse logic
 * stays exercised under the jsdom-based test runtime.
 */
export function useIsDesktop(): Ref<boolean> {
  return useMediaQuery('(min-width: 1024px)', true)
}
