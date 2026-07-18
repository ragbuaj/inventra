import { FIELD_CATALOG } from '~/constants/fieldCatalog'
import type { CellRule } from '~/constants/fieldCatalog'

export interface EntityView { key: string, fields: string[] }
export interface FieldRoleItem { id: string, code: string, name: string }
export interface FieldRow { entity: string, field: string, can_view: boolean, can_edit: boolean }
/** entity → field → rule; only EXPLICIT restriction cells are present. */
export type RoleRules = Record<string, Record<string, CellRule>>

interface RoleDTO { id: string, code: string, name: string }

// Stored rows → per-entity explicit rules. Entities outside FIELD_CATALOG are
// kept too, so a save round-trips rows this UI doesn't render.
export function rulesFromRows(rows: FieldRow[]): RoleRules {
  const out: RoleRules = {}
  for (const r of rows) {
    ;(out[r.entity] ??= {})[r.field] = { view: r.can_view, edit: r.can_edit }
  }
  return out
}

// Rules → restriction rows only. Full-allow cells are dropped: the backend
// stores restrictions and treats a missing row as default-allow.
export function rowsFromRules(rules: RoleRules): FieldRow[] {
  const rows: FieldRow[] = []
  for (const [entity, fields] of Object.entries(rules)) {
    for (const [field, cr] of Object.entries(fields)) {
      if (!(cr.view && cr.edit)) rows.push({ entity, field, can_view: cr.view, can_edit: cr.edit })
    }
  }
  return rows
}

/**
 * Field-permission rules, wired to /api/v1/authz. The catalog supplies the
 * maskable (entity, field) keys; a role's stored restrictions come lazily from
 * /authz/roles/:id/fields when that role is selected (no eager N+1 fan-out).
 * Default-allow: a cell with no stored policy is view+edit.
 */
export function useFieldPermission() {
  const { request } = useApiClient()

  function getEntities(): EntityView[] {
    return FIELD_CATALOG.map(e => ({ key: e.entity, fields: [...e.fields] }))
  }

  async function listRoles(): Promise<FieldRoleItem[]> {
    const res = await request<{ data: RoleDTO[], total: number }>('/authz/roles')
    return res.data.map(r => ({ id: r.id, code: r.code, name: r.name }))
  }

  async function getRoleRules(id: string): Promise<RoleRules> {
    const res = await request<{ fields: FieldRow[] }>(`/authz/roles/${id}/fields`)
    return rulesFromRows(res.fields)
  }

  async function saveRoleRules(id: string, rules: RoleRules): Promise<void> {
    await request(`/authz/roles/${id}/fields`, { method: 'PUT', body: { fields: rowsFromRules(rules) } })
  }

  return { getEntities, listRoles, getRoleRules, saveRoleRules }
}
