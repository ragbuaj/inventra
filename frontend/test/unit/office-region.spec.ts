import { describe, it, expect } from 'vitest'
import { wilayahAncestor, isInterRegion, type OfficeNode } from '~/utils/officeRegion'

// Tree:
//              pusat (pusat)
//             /            \
//      wilayah-1 (wilayah)   wilayah-2 (wilayah)
//       /        \                   \
// cabang-a   cabang-a2            cabang-b
//    |            |                   |
// unit-a       unit-a3             unit-b
//
// Plus a disconnected "ghost" node whose office_type_id has no known tier,
// and a two-node cycle (cyc-a <-> cyc-b) that never reaches a wilayah/root.
const NODES: OfficeNode[] = [
  { id: 'pusat', parent_id: null, office_type_id: 'pusat' },
  { id: 'wilayah-1', parent_id: 'pusat', office_type_id: 'wilayah' },
  { id: 'wilayah-2', parent_id: 'pusat', office_type_id: 'wilayah' },
  { id: 'cabang-a', parent_id: 'wilayah-1', office_type_id: 'cabang' },
  { id: 'cabang-a2', parent_id: 'wilayah-1', office_type_id: 'cabang' },
  { id: 'cabang-b', parent_id: 'wilayah-2', office_type_id: 'cabang' },
  { id: 'unit-a', parent_id: 'cabang-a', office_type_id: 'unit' },
  { id: 'unit-a3', parent_id: 'cabang-a2', office_type_id: 'unit' },
  { id: 'unit-b', parent_id: 'cabang-b', office_type_id: 'unit' },
  { id: 'ghost', parent_id: null, office_type_id: 'ghost-type' },
  { id: 'cyc-a', parent_id: 'cyc-b', office_type_id: 'unit' },
  { id: 'cyc-b', parent_id: 'cyc-a', office_type_id: 'unit' }
]

const nodeMap = new Map(NODES.map(n => [n.id, n]))

const TIER_BY_TYPE: Record<string, string | null | undefined> = {
  pusat: 'pusat',
  wilayah: 'wilayah',
  cabang: 'cabang',
  unit: 'unit'
}

function tierOf(officeTypeId: string): string | null | undefined {
  return TIER_BY_TYPE[officeTypeId]
}

describe('wilayahAncestor', () => {
  it.each([
    ['unit-a', 'wilayah-1'],
    ['cabang-a', 'wilayah-1'],
    ['unit-a3', 'wilayah-1'],
    ['unit-b', 'wilayah-2'],
    ['cabang-b', 'wilayah-2'],
    ['wilayah-1', 'wilayah-1']
  ])('climbs from %s to its wilayah ancestor %s', (start, expected) => {
    expect(wilayahAncestor(start, nodeMap, tierOf)).toBe(expected)
  })

  it('returns null when climbing reaches the root without finding a wilayah tier', () => {
    expect(wilayahAncestor('pusat', nodeMap, tierOf)).toBeNull()
  })

  it('returns null for a missing/unknown office id', () => {
    expect(wilayahAncestor('does-not-exist', nodeMap, tierOf)).toBeNull()
  })

  it('returns null when the tier lookup itself is unresolvable for every ancestor', () => {
    expect(wilayahAncestor('ghost', nodeMap, tierOf)).toBeNull()
  })

  it('returns null (cycle guard) instead of looping forever on a parent cycle', () => {
    expect(wilayahAncestor('cyc-a', nodeMap, tierOf)).toBeNull()
  })
})

describe('isInterRegion', () => {
  it('returns false for two offices resolving to the same wilayah', () => {
    expect(isInterRegion('unit-a', 'unit-a3', nodeMap, tierOf)).toBe(false)
  })

  it('returns false when comparing an office against itself', () => {
    expect(isInterRegion('unit-a', 'unit-a', nodeMap, tierOf)).toBe(false)
  })

  it('returns true for two offices resolving to different wilayah', () => {
    expect(isInterRegion('unit-a', 'unit-b', nodeMap, tierOf)).toBe(true)
  })

  it('returns null when one side is unresolvable (missing tier)', () => {
    expect(isInterRegion('unit-a', 'ghost', nodeMap, tierOf)).toBeNull()
  })

  it('returns null when one side is unresolvable (missing node)', () => {
    expect(isInterRegion('unit-a', 'does-not-exist', nodeMap, tierOf)).toBeNull()
  })

  it('returns null when one side is caught in a parent cycle', () => {
    expect(isInterRegion('unit-a', 'cyc-a', nodeMap, tierOf)).toBeNull()
  })
})
