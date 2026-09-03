import type { Ref } from 'vue'

/**
 * Resolves the app's scrolling container (`<main>` in `layouts/default.vue`,
 * which owns `overflow-y-auto` — the window itself never scrolls).
 *
 * The element is exposed as a ref for child props, but it is deliberately
 * **re-queried on every access** rather than captured once at mount: the
 * layout can replace its `<main>` while a page is still alive, and a detached
 * node keeps answering `scrollTop` with 0 and never intersects an
 * IntersectionObserver — both failing silently, with no error to notice.
 */
export function useScrollParent(): { el: Ref<HTMLElement | null>, get: () => HTMLElement | null } {
  const el = ref<HTMLElement | null>(null)

  function get(): HTMLElement | null {
    if (!import.meta.client) return null
    const found = document.querySelector('main') as HTMLElement | null
    if (found !== el.value) el.value = found
    return found
  }

  onMounted(get)

  return { el, get }
}
