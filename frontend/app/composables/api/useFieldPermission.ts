import { FIELD_CATALOG } from '~/constants/fieldCatalog'
import type { CellRule } from '~/constants/fieldCatalog'

export interface EntityView { key: string, fields: string[] }
export interface RoleColumn { key: string, label: string }
export interface FieldRow { entity: string, field: string, can_view: boolean, can_edit: boolean }
export type EntityRules = Record<string, Record<string, CellRule>>

interface RoleDTO { id: string, name: string }

// Derive an entity's restriction cells (field → roleId → rule) from all roles' rows.
export function deriveEntityRules(roleFields: Record<string, FieldRow[]>, entity: string): EntityRules {
  const out: EntityRules = {}
  for (const [roleId, rows] of Object.entries(roleFields)) {
    for (const r of rows) {
      if (r.entity !== entity) continue
      ;(out[r.field] ??= {})[roleId] = { view: r.can_view, edit: r.can_edit }
    }
  }
  return out
}

// Build a role's full field rows for a save: keep other-entity rows verbatim, then
// append only the RESTRICTION cells (not full-allow) of the target entity from `rules`.
export function buildRoleRows(existing: FieldRow[], entity: string, roleId: string, rules: EntityRules): FieldRow[] {
  const others = existing.filter(r => r.entity !== entity)
  const eRows: FieldRow[] = []
  for (const [field, perRole] of Object.entries(rules)) {
    const cr = perRole[roleId]
    if (cr && !(cr.view && cr.edit)) eRows.push({ entity, field, can_view: cr.view, can_edit: cr.edit })
  }
  return [...others, ...eRows]
}

// Order-insensitive comparison of a role's target-entity rows vs the edited `rules`.
export function entityRowsEqual(rows: FieldRow[], entity: string, rules: EntityRules, roleId: string): boolean {
  const cur = rows.filter(r => r.entity === entity)
  const next = buildRoleRows([], entity, roleId, rules)
  if (cur.length !== next.length) return false
  const key = (r: FieldRow) => `${r.field}:${r.can_view}:${r.can_edit}`
  const cs = new Set(cur.map(key))
  return next.every(r => cs.has(key(r)))
}

/**
 * Field-permission rules, wired to /api/v1/authz. The catalog supplies the
 * maskable (entity, field) keys; each role's policies come from
 * /authz/roles/:id/fields. Default-allow: a cell with no stored policy is
 * view+edit; only restriction cells are persisted.
 */
export function useFieldPermission() {
  const { request } = useApiClient()
  let roleFields: Record<string, FieldRow[]> = {}

  function getEntities(): EntityView[] {
    return FIELD_CATALOG.map(e => ({ key: e.entity, fields: [...e.fields] }))
  }

  async function load(): Promise<RoleColumn[]> {
    const res = await request<{ data: RoleDTO[], total: number }>('/authz/roles')
    const cols = res.data.map(r => ({ key: r.id, label: r.name }))
    const entries = await Promise.all(cols.map(async (c) => {
      const r = await request<{ fields: FieldRow[] }>(`/authz/roles/${c.key}/fields`)
      return [c.key, r.fields] as const
    }))
    roleFields = Object.fromEntries(entries)
    return cols
  }

  function getRules(entity: string): EntityRules {
    return deriveEntityRules(roleFields, entity)
  }

  async function saveRules(entity: string, rules: EntityRules, roleIds: string[]): Promise<void> {
    const changed = roleIds.filter(id => !entityRowsEqual(roleFields[id] ?? [], entity, rules, id))
    await Promise.all(changed.map((id) => {
      const next = buildRoleRows(roleFields[id] ?? [], entity, id, rules)
      return request(`/authz/roles/${id}/fields`, { method: 'PUT', body: { fields: next } })
        .then(() => { roleFields[id] = next })
    }))
  }

  return { getEntities, load, getRules, saveRules }
}
