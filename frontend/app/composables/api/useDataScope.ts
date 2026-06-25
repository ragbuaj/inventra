import type { ScopeModuleDef, ScopeRole } from '~/mock/dataScope'
import { DATA_SCOPE_MODULES, dataScopeStore } from '~/mock/dataScope'
import { fakeLatency } from '~/mock/helpers'

type Locale = 'id' | 'en'

export interface ScopeModuleView {
  key: string
  label: string
}

export interface ScopeRoleView {
  key: string
  nama: string
  sub: string
  def: ScopeRole['def']
  ov: Record<string, ScopeRole['def']>
}

function resolveRole(r: ScopeRole, locale: Locale): ScopeRoleView {
  return {
    key: r.key,
    nama: r.nama[locale] ?? r.nama.id,
    sub: r.sub[locale] ?? r.sub.id,
    def: r.def,
    ov: { ...r.ov }
  }
}

/**
 * Data-scope policies. Mock-first; the seam a real implementation swaps behind
 * (`/auth/data-scope-policies`). The module catalog is static metadata, so `getModules` is sync.
 */
export function useDataScope() {
  function getModules(locale: Locale = 'id'): ScopeModuleView[] {
    return DATA_SCOPE_MODULES.map((m: ScopeModuleDef) => ({ key: m.key, label: m.label[locale] ?? m.label.id }))
  }

  async function listRoles(locale: Locale = 'id'): Promise<ScopeRoleView[]> {
    await fakeLatency()
    return dataScopeStore.all().map(r => resolveRole(r, locale))
  }

  async function saveScopes(roleViews: ScopeRoleView[]): Promise<void> {
    await fakeLatency()
    dataScopeStore.replace(roleViews.map(r => ({
      key: r.key,
      nama: { id: r.nama, en: r.nama },
      sub: { id: r.sub, en: r.sub },
      def: r.def,
      ov: { ...r.ov }
    })))
  }

  return { getModules, listRoles, saveScopes }
}
