import type { User, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { userStore } from '~/mock/users'

export interface UserInput {
  nama: string
  email: string
  /** write-only; never stored on the row (a real API would hash it) */
  password?: string
  peran: string
  kantor: string
  pegawai: string
  login: User['login']
  status: User['status']
}

/** Strip the write-only password before it touches the store. */
function toRow(input: UserInput): Omit<User, 'id' | 'created_at'> {
  const { password: _password, ...rest } = input
  return rest
}

export function useUsers() {
  async function list(query: ListQuery = {}): Promise<Paginated<User>> {
    await fakeLatency()
    return paginate(filterBy(userStore.all(), query, ['nama', 'email']), query)
  }

  async function get(id: string): Promise<User | undefined> {
    await fakeLatency()
    return userStore.find(id)
  }

  async function create(input: UserInput): Promise<User> {
    await fakeLatency()
    return userStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...toRow(input) })
  }

  async function update(id: string, input: UserInput): Promise<User> {
    await fakeLatency()
    const row = userStore.patch(id, toRow(input))
    if (!row) throw new Error('settings.users.errNotFound')
    return row
  }

  async function setStatus(id: string, status: User['status']): Promise<User> {
    await fakeLatency()
    const row = userStore.patch(id, { status })
    if (!row) throw new Error('settings.users.errNotFound')
    return row
  }

  /** Mock reset-password seam — no email/token flow yet. */
  async function resetPassword(_id: string): Promise<void> {
    await fakeLatency()
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    userStore.remove(id)
  }

  return { list, get, create, update, setStatus, resetPassword, remove }
}
