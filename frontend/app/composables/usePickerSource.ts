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

function referenceRowToItem(row: ReferenceRow): PickerItem {
  return { id: row.id, label: row.name, sublabel: row.code }
}

/**
 * Generic reference-resource adapter for AsyncSearchPicker — label = name, sublabel = code
 * when present. `useReference()` only exposes list/create/update/remove (no per-id getter),
 * even though the backend's generic reference engine serves GET /<resource>/:id (see
 * backend/internal/masterdata/reference/routes.go). resolveFn hits that endpoint directly
 * via useApiClient() until useReference() grows a get().
 */
export function useReferencePicker(resource: ReferenceKey) {
  const api = useReference()
  const { request } = useApiClient()
  return {
    async searchFn(term: string): Promise<PickerItem[]> {
      const res = await api.list(resource, { search: term, limit: 20 })
      return res.data.map(referenceRowToItem)
    },
    async resolveFn(id: string): Promise<PickerItem | null> {
      try {
        const row = await request<ReferenceRow>(`/${resource}/${id}`)
        return referenceRowToItem(row)
      } catch {
        return null
      }
    }
  }
}
