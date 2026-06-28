import { iconForGroup } from '~/constants/authzCatalog'

export interface PermissionView {
  code: string
  label: string
}

export interface ModuleView {
  key: string
  label: string
  icon: string
  perms: PermissionView[]
}

export interface RoleView {
  id: string
  code: string
  name: string
  is_system: boolean
  description?: string
  perms: string[]
}

export interface CreateRoleInput {
  name: string
  description?: string
  copyFromId?: string
}

interface CatalogResponse {
  permissions: { group: string, items: { key: string, label: string }[] }[]
}

interface RoleDTO {
  id: string
  code: string
  name: string
  is_system: boolean
  description?: string
}

// slugifyRoleCode derives a backend role `code` from a human name:
// lowercase, runs of non-alphanumerics collapse to a single '_', trimmed.
export function slugifyRoleCode(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
}

/**
 * RBAC data source, wired to /api/v1/authz. The catalog supplies the
 * authoritative permission key set + grouping; display labels are resolved by
 * the UI via i18n (with fallback to the catalog label), icons via iconForGroup.
 */
export function useRbac() {
  const { request } = useApiClient()

  async function getCatalog(): Promise<ModuleView[]> {
    const cat = await request<CatalogResponse>('/authz/catalog')
    return cat.permissions.map(g => ({
      key: g.group,
      label: g.group,
      icon: iconForGroup(g.group),
      perms: g.items.map(i => ({ code: i.key, label: i.label }))
    }))
  }

  async function listRoles(): Promise<RoleView[]> {
    const res = await request<{ data: RoleDTO[], total: number }>('/authz/roles')
    return res.data.map(r => ({ ...r, perms: [] }))
  }

  async function getRolePermissions(id: string): Promise<string[]> {
    const res = await request<{ permissions: string[] }>(`/authz/roles/${id}/permissions`)
    return res.permissions
  }

  async function updateRolePermissions(id: string, perms: string[]): Promise<void> {
    await request(`/authz/roles/${id}/permissions`, { method: 'PUT', body: { permissions: perms } })
  }

  async function createRole(input: CreateRoleInput): Promise<RoleView> {
    let copied: string[] = []
    if (input.copyFromId) copied = await getRolePermissions(input.copyFromId)
    const role = await request<RoleDTO>('/authz/roles', {
      method: 'POST',
      body: { code: slugifyRoleCode(input.name), name: input.name.trim(), description: input.description?.trim() || undefined }
    })
    if (copied.length) await updateRolePermissions(role.id, copied)
    return { ...role, perms: copied }
  }

  return { getCatalog, listRoles, getRolePermissions, createRole, updateRolePermissions }
}
