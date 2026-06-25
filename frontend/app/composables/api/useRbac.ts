import type { ModuleDef, Role } from '~/mock/rbac'
import { RBAC_MODULES, roleStore } from '~/mock/rbac'
import { fakeLatency } from '~/mock/helpers'

type Locale = 'id' | 'en'

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
  key: string
  nama: string
  system: boolean
  desc: string
  perms: string[]
}

export interface CreateRoleInput {
  nama: string
  copyFromKey?: string
  desc?: string
}

let seq = 1

function resolveModule(m: ModuleDef, locale: Locale): ModuleView {
  return {
    key: m.key,
    label: m.label[locale] ?? m.label.id,
    icon: m.icon,
    perms: m.perms.map(p => ({ code: p.code, label: p.label[locale] ?? p.label.id }))
  }
}

function resolveRole(r: Role, locale: Locale): RoleView {
  return {
    key: r.key,
    nama: r.nama[locale] ?? r.nama.id,
    system: r.system,
    desc: r.desc[locale] ?? r.desc.id,
    perms: [...r.perms]
  }
}

/**
 * RBAC data source. Mock-first; the seam a real implementation swaps behind
 * (`/auth/roles`, `/auth/role-permissions`). The module/permission catalog is static metadata, so
 * `getModules` is synchronous; role reads/writes go through the mock store.
 */
export function useRbac() {
  function getModules(locale: Locale = 'id'): ModuleView[] {
    return RBAC_MODULES.map(m => resolveModule(m, locale))
  }

  async function listRoles(locale: Locale = 'id'): Promise<RoleView[]> {
    await fakeLatency()
    return roleStore.all().map(r => resolveRole(r, locale))
  }

  async function createRole(input: CreateRoleInput, locale: Locale = 'id'): Promise<RoleView> {
    await fakeLatency()
    const base = input.copyFromKey ? (roleStore.find(input.copyFromKey)?.perms ?? []) : []
    const nama = input.nama.trim()
    const desc = input.desc?.trim() || (locale === 'en' ? 'Custom role.' : 'Peran kustom.')
    const key = `custom-${seq++}`
    const role: Role = {
      key,
      nama: { id: nama, en: nama },
      system: false,
      desc: { id: desc, en: desc },
      perms: [...base]
    }
    roleStore.insert(role)
    return resolveRole(role, locale)
  }

  async function updateRolePermissions(key: string, perms: string[]): Promise<void> {
    await fakeLatency()
    roleStore.setPerms(key, perms)
  }

  return { getModules, listRoles, createRole, updateRolePermissions }
}
