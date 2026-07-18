import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  useFieldPermission, rulesFromRows, rowsFromRules
} from '~/composables/api/useFieldPermission'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

beforeEach(() => request.mockReset())

describe('pure helpers', () => {
  const rows = [
    { entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false },
    { entity: 'users', field: 'email', can_view: true, can_edit: false }
  ]

  it('rulesFromRows maps stored rows to entity→field→rule', () => {
    expect(rulesFromRows(rows)).toEqual({
      assets: { purchase_cost: { view: false, edit: false } },
      users: { email: { view: true, edit: false } }
    })
  })

  it('rowsFromRules keeps only restriction cells and drops full-allow', () => {
    const out = rowsFromRules({
      assets: {
        purchase_cost: { view: true, edit: true }, // full-allow → dropped
        book_value: { view: false, edit: false }
      },
      users: { email: { view: true, edit: false } }
    })
    expect(out).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
    expect(out).toContainEqual({ entity: 'users', field: 'email', can_view: true, can_edit: false })
    expect(out.find(r => r.field === 'purchase_cost')).toBeUndefined()
  })

  it('rulesFromRows/rowsFromRules round-trip restriction rows', () => {
    expect(rowsFromRules(rulesFromRows(rows))).toEqual(expect.arrayContaining(rows))
  })
})

describe('useFieldPermission', () => {
  it('getEntities comes from the catalog', () => {
    const ents = useFieldPermission().getEntities()
    expect(ents.map(e => e.key)).toEqual(['assets', 'users', 'requests', 'employees'])
  })

  it('listRoles is a single GET (no per-role fan-out)', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'r1', code: 'manager', name: 'Manager' }], total: 1 })
    const roles = await useFieldPermission().listRoles()
    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith('/authz/roles')
    expect(roles).toEqual([{ id: 'r1', code: 'manager', name: 'Manager' }])
  })

  it('getRoleRules fetches one role lazily and derives rules', async () => {
    request.mockResolvedValueOnce({ fields: [{ entity: 'assets', field: 'purchase_cost', can_view: false, can_edit: false }] })
    const rules = await useFieldPermission().getRoleRules('r1')
    expect(request).toHaveBeenCalledWith('/authz/roles/r1/fields')
    expect(rules).toEqual({ assets: { purchase_cost: { view: false, edit: false } } })
  })

  it('saveRoleRules PUTs restriction rows for all entities of the role', async () => {
    request.mockResolvedValueOnce({})
    await useFieldPermission().saveRoleRules('r1', {
      assets: { book_value: { view: false, edit: false } },
      users: { email: { view: true, edit: false } }
    })
    expect(request).toHaveBeenCalledTimes(1)
    const [path, opts] = request.mock.calls[0]!
    expect(path).toBe('/authz/roles/r1/fields')
    expect(opts.method).toBe('PUT')
    expect(opts.body.fields).toContainEqual({ entity: 'assets', field: 'book_value', can_view: false, can_edit: false })
    expect(opts.body.fields).toContainEqual({ entity: 'users', field: 'email', can_view: true, can_edit: false })
  })
})
