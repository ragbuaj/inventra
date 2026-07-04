import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  useFieldPermission, deriveEntityRules, buildRoleRows, entityRowsEqual
} from '~/composables/api/useFieldPermission'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

describe('pure helpers', () => {
  const roleFields = {
    r1: [
      { entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false },
      { entity: 'users', field: 'email', can_view: true, can_edit: false }
    ],
    r2: []
  }

  it('deriveEntityRules keeps only the entity, as field→role→rule', () => {
    expect(deriveEntityRules(roleFields, 'assets')).toEqual({
      purchase_cost: { r1: { view: false, edit: false } }
    })
    expect(deriveEntityRules(roleFields, 'users')).toEqual({
      email: { r1: { view: true, edit: false } }
    })
  })

  it('buildRoleRows preserves other entities + keeps only restriction cells of the target entity', () => {
    const rules = { purchase_cost: { r1: { view: true, edit: true } }, book_value: { r1: { view: false, edit: false } } }
    const rows = buildRoleRows(roleFields.r1, 'assets', 'r1', rules)
    // users/email (other entity) preserved; purchase_cost is full-allow → dropped; book_value restriction kept
    expect(rows).toContainEqual({ entity: 'users', field: 'email', can_view: true, can_edit: false })
    expect(rows).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
    expect(rows.find(r => r.entity === 'assets' && r.field === 'purchase_cost')).toBeUndefined()
  })

  it('entityRowsEqual detects changes for the target entity only', () => {
    const same = { purchase_cost: { r1: { view: false, edit: false } } }
    expect(entityRowsEqual(roleFields.r1, 'assets', same, 'r1')).toBe(true)
    const changed = { purchase_cost: { r1: { view: true, edit: false } } }
    expect(entityRowsEqual(roleFields.r1, 'assets', changed, 'r1')).toBe(false)
  })
})

describe('useFieldPermission', () => {
  it('getEntities comes from the catalog', () => {
    const ents = useFieldPermission().getEntities()
    expect(ents.map(e => e.key)).toEqual(['assets', 'users', 'requests'])
  })

  it('load fetches roles then each role fields; getRules derives restrictions', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    const cols = await fp.load()
    expect(request).toHaveBeenNthCalledWith(1, '/authz/roles')
    expect(request).toHaveBeenNthCalledWith(2, '/authz/roles/r1/fields')
    expect(cols).toEqual([{ key: 'r1', label: 'Manager' }])
    expect(fp.getRules('assets')).toEqual({ purchase_cost: { r1: { view: false, edit: false } } })
  })

  it('saveRules PUTs only changed roles with reconstructed full rows', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'users', field: 'email', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    await fp.load()
    request.mockClear()
    request.mockResolvedValueOnce({ fields: [] })
    // add an assets restriction; users/email must be preserved in the PUT body
    await fp.saveRules('assets', { book_value: { r1: { view: false, edit: false } } }, ['r1'])
    expect(request).toHaveBeenCalledTimes(1)
    const [path, opts] = request.mock.calls[0]
    expect(path).toBe('/authz/roles/r1/fields')
    expect(opts.method).toBe('PUT')
    expect(opts.body.fields).toContainEqual({ entity: 'users', field: 'email', can_view: false, can_edit: false })
    expect(opts.body.fields).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
  })

  it('saveRules PUTs nothing when the entity is unchanged', async () => {
    request
      .mockResolvedValueOnce({ data: [{ id: 'r1', name: 'Manager' }], total: 1 })
      .mockResolvedValueOnce({ fields: [{ entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false }] })
    const fp = useFieldPermission()
    await fp.load()
    request.mockClear()
    await fp.saveRules('assets', { purchase_cost: { r1: { view: false, edit: false } } }, ['r1'])
    expect(request).not.toHaveBeenCalled()
  })
})
