import { describe, it, expect } from 'vitest'
import { AUDIT_ENTITY_TYPES } from '~/constants/auditCatalog'

describe('AUDIT_ENTITY_TYPES', () => {
  it('lists the real recorded entity types', () => {
    expect(AUDIT_ENTITY_TYPES).toContain('assets')
    expect(AUDIT_ENTITY_TYPES).toContain('users')
    expect(AUDIT_ENTITY_TYPES).toContain('roles')
    expect(AUDIT_ENTITY_TYPES).toContain('field_permissions')
  })
  it('has no duplicates', () => {
    expect(new Set(AUDIT_ENTITY_TYPES).size).toBe(AUDIT_ENTITY_TYPES.length)
  })
})
