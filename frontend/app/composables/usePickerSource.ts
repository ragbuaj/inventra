import type { PickerItem, ReferenceRow } from '~/types'
import type { ReferenceKey } from '~/composables/api/referenceResources'

/** Office adapter for AsyncSearchPicker — label = name, sublabel = code. */
export function useOfficePicker() {
  const api = useOffices()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(o => ({ id: o.id, label: o.name, sublabel: o.code }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const o = await api.get(id)
        return { id: o.id, label: o.name, sublabel: o.code }
      } catch {
        return null
      }
    }
  }
}

/** Employee adapter for AsyncSearchPicker — label = name, sublabel = code. */
export function useEmployeePicker() {
  const api = useEmployees()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(e => ({ id: e.id, label: e.name, sublabel: e.code }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const e = await api.get(id)
        return { id: e.id, label: e.name, sublabel: e.code }
      } catch {
        return null
      }
    }
  }
}

/**
 * Category adapter for AsyncSearchPicker — label = name, sublabel = code.
 * useCategories() already exposes get(id) (unlike useReference()), so
 * resolveFn hits it directly — no reach-around needed.
 */
export function useCategoryPicker() {
  const api = useCategories()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20 })
      return res.data.map(c => ({ id: c.id, label: c.name, sublabel: c.code ?? undefined }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const c = await api.get(id)
        return { id: c.id, label: c.name, sublabel: c.code ?? undefined }
      } catch {
        return null
      }
    }
  }
}

function referenceRowToItem(row: ReferenceRow): PickerItem {
  return { id: row.id, label: row.name, sublabel: row.code }
}

/** Generic reference-resource adapter for AsyncSearchPicker — label = name, sublabel = code when present. */
export function useReferencePicker(resource: ReferenceKey) {
  const api = useReference()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list(resource, { search: term, limit: 20 })
      return res.data.map(referenceRowToItem)
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const row = await api.get(resource, id)
        return referenceRowToItem(row)
      } catch {
        return null
      }
    }
  }
}

/**
 * User adapter for AsyncSearchPicker — label = name, sublabel = email.
 * useUsers() exposes no per-id getter, so resolveFn reaches around via
 * useApiClient() directly.
 */
export function useUserPicker() {
  const api = useUsers()
  const { request } = useApiClient()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list({ search: term, limit: 20, offset: 0 })
      return res.rows.map(u => ({ id: u.id, label: u.name, sublabel: u.email }))
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const u = await request<{ id: string, name: string, email: string }>(`/users/${id}`)
        return { id: u.id, label: u.name, sublabel: u.email }
      } catch {
        return null
      }
    }
  }
}
