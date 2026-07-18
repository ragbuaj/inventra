import type { ScopeLevel } from '~/constants/dataScope'

export interface ScopeModuleView {
  key: string
}

export interface ScopeRoleItem {
  id: string
  code: string
  name: string
  sub: string
}

export interface RoleScopeView {
  def: ScopeLevel
  ov: Record<string, ScopeLevel>
}

interface CatalogResponse {
  scope_modules: string[]
}

interface RoleDTO {
  id: string
  code: string
  name: string
  description?: string
}

interface PolicyItem {
  module: string
  scope_level: ScopeLevel
}

interface ScopeResponse {
  policies: PolicyItem[]
}

/**
 * Data-scope policies, wired to /api/v1/authz. Module rows come from the
 * catalog's scope_modules. The roles list is a single GET /authz/roles; a
 * role's default (module "*") + per-module overrides are fetched lazily per
 * role via getRoleScope when that role is selected (no eager N+1 fan-out).
 */
export function useDataScope() {
  const { request } = useApiClient()

  async function getModules(): Promise<ScopeModuleView[]> {
    const cat = await request<CatalogResponse>('/authz/catalog')
    return cat.scope_modules.filter(m => m !== '*').map(key => ({ key }))
  }

  async function listRoles(): Promise<ScopeRoleItem[]> {
    const res = await request<{ data: RoleDTO[], total: number }>('/authz/roles')
    return res.data.map(r => ({ id: r.id, code: r.code, name: r.name, sub: r.description ?? '' }))
  }

  async function getRoleScope(id: string): Promise<RoleScopeView> {
    const sc = await request<ScopeResponse>(`/authz/roles/${id}/scope`)
    const def: ScopeLevel = sc.policies.find(p => p.module === '*')?.scope_level ?? 'own'
    const ov: Record<string, ScopeLevel> = {}
    for (const p of sc.policies) {
      if (p.module !== '*') ov[p.module] = p.scope_level
    }
    return { def, ov }
  }

  async function saveRoleScope(id: string, def: ScopeLevel, ov: Record<string, ScopeLevel>): Promise<void> {
    const policies = [
      { module: '*', scope_level: def },
      ...Object.entries(ov).map(([module, scope_level]) => ({ module, scope_level }))
    ]
    await request(`/authz/roles/${id}/scope`, { method: 'PUT', body: { policies } })
  }

  return { getModules, listRoles, getRoleScope, saveRoleScope }
}
