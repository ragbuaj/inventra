import type { ListQuery, Office, Paginated, TreeNode } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { buildOfficeTree, officeStore } from '~/mock/offices'

export interface OfficeInput {
  nama: string
  kode: string
  tipe: Office['tipe']
  parent_id: string | null
  provinsi: string
  kota: string
  alamat: string
  active?: boolean
}

function assertValidParent(parentId: string | null) {
  if (parentId && !officeStore.find(parentId)) {
    throw new Error('masterdata.offices.errInvalidParent')
  }
}

export function useOffices() {
  async function list(query: ListQuery = {}): Promise<Paginated<Office>> {
    await fakeLatency()
    return paginate(filterBy(officeStore.all(), query, ['nama', 'kode', 'kota']), query)
  }

  async function get(id: string): Promise<Office | undefined> {
    await fakeLatency()
    return officeStore.find(id)
  }

  async function tree(): Promise<TreeNode[]> {
    await fakeLatency()
    return buildOfficeTree(officeStore.all())
  }

  async function create(input: OfficeInput): Promise<Office> {
    await fakeLatency()
    assertValidParent(input.parent_id)
    return officeStore.insert({ id: generateId(), created_at: new Date().toISOString(), active: true, ...input })
  }

  async function update(id: string, input: OfficeInput): Promise<Office> {
    await fakeLatency()
    assertValidParent(input.parent_id)
    const row = officeStore.patch(id, input)
    if (!row) throw new Error('masterdata.offices.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    officeStore.remove(id)
  }

  return { list, get, tree, create, update, remove }
}
