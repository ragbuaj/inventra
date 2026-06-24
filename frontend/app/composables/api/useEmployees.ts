import type { Employee, ListQuery, Paginated } from '~/types'
import { fakeLatency, filterBy, generateId, paginate } from '~/mock/helpers'
import { employeeStore } from '~/mock/employees'

export interface EmployeeInput {
  nip: string
  nama: string
  email: string
  telepon: string
  jabatan: string
  departemen: string
  office_id: string
  status: Employee['status']
}

export function useEmployees() {
  async function list(query: ListQuery = {}): Promise<Paginated<Employee>> {
    await fakeLatency()
    return paginate(filterBy(employeeStore.all(), query, ['nama', 'nip', 'email']), query)
  }

  async function get(id: string): Promise<Employee | undefined> {
    await fakeLatency()
    return employeeStore.find(id)
  }

  async function create(input: EmployeeInput): Promise<Employee> {
    await fakeLatency()
    return employeeStore.insert({ id: generateId(), created_at: new Date().toISOString(), ...input })
  }

  async function update(id: string, input: EmployeeInput): Promise<Employee> {
    await fakeLatency()
    const row = employeeStore.patch(id, input)
    if (!row) throw new Error('masterdata.employees.errNotFound')
    return row
  }

  async function remove(id: string): Promise<void> {
    await fakeLatency()
    employeeStore.remove(id)
  }

  return { list, get, create, update, remove }
}
