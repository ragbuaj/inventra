import { describe, it, expect } from 'vitest'
import { buildOfficeTree } from '~/mock/offices'
import type { Office } from '~/types'

function office(id: string, nama: string, parent_id: string | null, active = true): Office {
  return { id, nama, kode: id, tipe: 'cabang', parent_id, provinsi: 'X', kota: 'Y', alamat: 'Z', active, created_at: '2026-01-01' }
}

describe('buildOfficeTree', () => {
  it('nests children under their parent', () => {
    const tree = buildOfficeTree([
      office('1', 'Pusat', null),
      office('2', 'Kanwil A', '1'),
      office('3', 'Cabang A1', '2')
    ])
    expect(tree).toHaveLength(1)
    expect(tree[0].label).toBe('Pusat')
    expect(tree[0].children?.[0].label).toBe('Kanwil A')
    expect(tree[0].children?.[0].children?.[0].label).toBe('Cabang A1')
  })

  it('reports child counts and leaves children undefined for leaves', () => {
    const tree = buildOfficeTree([
      office('1', 'Pusat', null),
      office('2', 'Kanwil A', '1')
    ])
    expect(tree[0].childCount).toBe(1)
    expect(tree[0].children?.[0].children).toBeUndefined()
  })

  it('returns multiple roots when several offices have no parent', () => {
    const tree = buildOfficeTree([office('1', 'A', null), office('2', 'B', null)])
    expect(tree).toHaveLength(2)
  })

  it('sets inactive=true on tree nodes for inactive offices', () => {
    const tree = buildOfficeTree([
      office('1', 'Pusat', null, true),
      office('2', 'Cabang Nonaktif', '1', false)
    ])
    expect(tree[0].inactive).toBeFalsy()
    expect(tree[0].children?.[0].inactive).toBe(true)
  })

  it('exposes iconBg and iconColor on tree nodes', () => {
    const tree = buildOfficeTree([office('1', 'Pusat', null, true)])
    // cabang tipe gets amber tokens
    expect(tree[0].iconBg).toBeDefined()
    expect(tree[0].iconColor).toBeDefined()
  })
})
