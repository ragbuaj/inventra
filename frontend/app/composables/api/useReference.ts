import type { ListQuery, Paginated, ReferenceRow } from '~/types'
import type { ReferenceKey } from './referenceResources'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { referenceStores } from '~/mock/reference'

export function useReference() {
  async function list(key: ReferenceKey, query: ListQuery = {}): Promise<Paginated<ReferenceRow>> {
    await fakeLatency()
    return paginate(filterBy(referenceStores[key].all(), query, ['name', 'code']), query)
  }

  async function create(key: ReferenceKey, input: Record<string, unknown>): Promise<ReferenceRow> {
    await fakeLatency()
    return referenceStores[key].insert({ id: generateId(), name: '', ...input } as ReferenceRow)
  }

  async function update(key: ReferenceKey, id: string, input: Record<string, unknown>): Promise<ReferenceRow> {
    await fakeLatency()
    const row = referenceStores[key].patch(id, input as Partial<ReferenceRow>)
    if (!row) throw new Error('masterdata.reference.errNotFound')
    return row
  }

  async function remove(key: ReferenceKey, id: string): Promise<void> {
    await fakeLatency()
    referenceStores[key].remove(id)
  }

  return { list, create, update, remove }
}
