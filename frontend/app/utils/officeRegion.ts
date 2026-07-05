/**
 * Pure office-hierarchy helpers shared by the Mutasi (transfer) screens to
 * detect inter-region ("antar-wilayah") moves, which require Kepala Kanwil
 * approval on both sides. Kept framework-free so the climb/cycle logic is
 * unit-testable without mounting or a live office tree.
 */

export interface OfficeNode {
  id: string
  parent_id: string | null
  office_type_id: string
}

/**
 * Climbs the parent chain from `officeId` to the nearest ancestor (inclusive)
 * whose office-type tier is `'wilayah'`. Returns null when unresolvable: the
 * starting office (or an ancestor) is missing from `nodes`, no ancestor has
 * the wilayah tier before the chain ends, or the chain cycles back on itself.
 */
export function wilayahAncestor(
  officeId: string,
  nodes: Map<string, OfficeNode>,
  tierOf: (officeTypeId: string) => string | null | undefined
): string | null {
  const visited = new Set<string>()
  let currentId: string | null = officeId

  while (currentId !== null) {
    if (visited.has(currentId)) return null
    visited.add(currentId)

    const node = nodes.get(currentId)
    if (!node) return null

    if (tierOf(node.office_type_id) === 'wilayah') return node.id

    currentId = node.parent_id
  }

  return null
}

/**
 * true = `a` and `b` resolve to different wilayah ancestors (inter-region).
 * false = they resolve to the same wilayah ancestor (same region).
 * null = either side is unresolvable — callers should render no alert rather
 * than guess.
 */
export function isInterRegion(
  a: string,
  b: string,
  nodes: Map<string, OfficeNode>,
  tierOf: (id: string) => string | null | undefined
): boolean | null {
  const wilayahA = wilayahAncestor(a, nodes, tierOf)
  const wilayahB = wilayahAncestor(b, nodes, tierOf)
  if (wilayahA === null || wilayahB === null) return null
  return wilayahA !== wilayahB
}
