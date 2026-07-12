import type { PickerItem } from '~/types'

/**
 * Small on-demand id→label cache for table cells that used to resolve names
 * off an eagerly-fetched `{ limit: 100 }` array. Backed by a picker adapter's
 * `resolveFn` (see `usePickerSource.ts`) — the first `get(id)` for a given id
 * triggers a background resolve (memoized, de-duped via a pending set) and
 * returns '—' until it settles; the reactive Map then drives a re-render with
 * the resolved label. Unknown/failed ids fall back to the raw id (matches the
 * previous eager id→name map's `?? id` fallback).
 */
export function useResolveCache(resolveFn: (id: string) => Promise<PickerItem | null>) {
  const cache = ref(new Map<string, string>())
  const pending = new Set<string>()

  function warm(id: string | null | undefined) {
    if (!id || cache.value.has(id) || pending.has(id)) return
    pending.add(id)
    resolveFn(id).then((item) => {
      cache.value.set(id, item?.label ?? id)
    }).finally(() => {
      pending.delete(id)
    })
  }

  function get(id: string | null | undefined): string {
    if (!id) return '—'
    warm(id)
    return cache.value.get(id) ?? '—'
  }

  return { get, warm }
}
