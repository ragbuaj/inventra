import { describe, it, expect } from 'vitest'
import { FIELD_CATALOG } from '~/constants/fieldCatalog'

describe('FIELD_CATALOG', () => {
  it('lists the real backend-enforced entities', () => {
    expect(FIELD_CATALOG.map(e => e.entity)).toEqual(['assets', 'users', 'requests', 'employees'])
  })
  it('includes the employees entity with its maskable fields', () => {
    const emp = FIELD_CATALOG.find(e => e.entity === 'employees')
    expect(emp).toBeTruthy()
    expect(emp!.fields).toEqual(['name', 'email', 'phone', 'department_id', 'position_id', 'office_id', 'status'])
  })
  it('uses real serialization field keys (no Indonesian mock codes)', () => {
    const assets = FIELD_CATALOG.find(e => e.entity === 'assets')!
    expect(assets.fields).toContain('purchase_cost')
    expect(assets.fields).toContain('book_value')
    expect(assets.fields).not.toContain('harga_beli')
    const users = FIELD_CATALOG.find(e => e.entity === 'users')!
    expect(users.fields).toContain('email')
  })
  it('has no duplicate fields within an entity', () => {
    for (const e of FIELD_CATALOG) {
      expect(new Set(e.fields).size).toBe(e.fields.length)
    }
  })
})
