import type { Category, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { categoryStore } from '~/mock/categories'

export type CategoryInput = Omit<Category, 'id' | 'created_at'>

export function useCategories() {
  async function list(query: ListQuery = {}): Promise<Paginated<Category>> {
    await fakeLatency()
    return paginate(filterBy(categoryStore.all(), query, ['name', 'code']), query)
  }

  async function get(id: string): Promise<Category | undefined> {
    await fakeLatency()
    return categoryStore.find(id)
  }

  async function create(input: CategoryInput): Promise<Category> {
    await fakeLatency()
    return categoryStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...input })
  }

  async function update(id: string, input: CategoryInput): Promise<Category> {
    await fakeLatency()
    const row = categoryStore.patch(id, input)
    if (!row) throw new Error('masterdata.categories.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    categoryStore.remove(id)
  }

  return { list, get, create, update, remove }
}
