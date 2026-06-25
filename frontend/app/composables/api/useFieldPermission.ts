import type { EntityRules, FieldDef } from '~/mock/fieldPermission'
import { FIELD_ENTITIES, FIELD_ROLE_KEYS, FIELD_ROLE_LABELS, fieldPermStore } from '~/mock/fieldPermission'
import { fakeLatency } from '~/mock/helpers'

type Locale = 'id' | 'en'

export interface FieldView {
  code: string
  label: string
}

export interface EntityView {
  key: string
  label: string
  fields: FieldView[]
}

export interface RoleColumn {
  key: string
  label: string
}

/**
 * Field-permission rules. Mock-first; the seam a real implementation swaps behind
 * (`/auth/field-permissions`). Entity/field/role metadata is static, so those getters are sync.
 */
export function useFieldPermission() {
  function getEntities(locale: Locale = 'id'): EntityView[] {
    return FIELD_ENTITIES.map(e => ({
      key: e.key,
      label: e.label[locale] ?? e.label.id,
      fields: e.fields.map((fl: FieldDef) => ({ code: fl.code, label: fl.label[locale] ?? fl.label.id }))
    }))
  }

  function getRoleColumns(locale: Locale = 'id'): RoleColumn[] {
    return FIELD_ROLE_KEYS.map((k) => {
      const lbl = FIELD_ROLE_LABELS[k]
      return { key: k, label: lbl ? (lbl[locale] ?? lbl.id) : k }
    })
  }

  async function getRules(entityKey: string): Promise<EntityRules> {
    await fakeLatency()
    return fieldPermStore.get(entityKey)
  }

  async function saveRules(entityKey: string, rules: EntityRules): Promise<void> {
    await fakeLatency()
    fieldPermStore.set(entityKey, rules)
  }

  return { getEntities, getRoleColumns, getRules, saveRules }
}
