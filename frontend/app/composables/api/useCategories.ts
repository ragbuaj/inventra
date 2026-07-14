import type { Category, ListQuery, Paginated } from '~/types'

export type CategoryInput = Omit<Category, 'id' | 'created_at' | 'updated_at'>

/** Asset categories, wired to /api/v1/categories. `tree()` loads the full set. */
export function useCategories() {
  const { request } = useApiClient()

  async function list(query: ListQuery = {}): Promise<Paginated<Category>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 10))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<Category>>(`/categories?${q.toString()}`)
  }

  async function tree(): Promise<Category[]> {
    const res = await request<{ data: Category[] }>('/categories/tree')
    return res.data
  }

  async function get(id: string): Promise<Category> {
    return request<Category>(`/categories/${id}`)
  }

  async function create(input: CategoryInput): Promise<Category> {
    return request<Category>('/categories', { method: 'POST', body: input })
  }

  async function update(id: string, input: CategoryInput): Promise<Category> {
    return request<Category>(`/categories/${id}`, { method: 'PUT', body: input })
  }

  async function remove(id: string): Promise<void> {
    await request(`/categories/${id}`, { method: 'DELETE' })
  }

  return { list, get, create, update, remove, tree }
}
