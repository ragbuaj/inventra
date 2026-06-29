import type { Employee } from '~/types'
import { createStore } from './helpers'

export const employeeSeed: Employee[] = [
  { id: 'e-1', code: '199001012015011001', name: 'Andi Pratama', email: 'andi.pratama@inventra.go.id', phone: '0812-1111-2222', position_id: null, department_id: null, office_id: 'o-jkt', status: 'active', created_at: '2026-01-10', updated_at: '2026-01-10' },
  { id: 'e-2', code: '199203122016012002', name: 'Bunga Lestari', email: 'bunga.lestari@inventra.go.id', phone: '0813-3333-4444', position_id: null, department_id: null, office_id: 'o-jkt', status: 'active', created_at: '2026-01-11', updated_at: '2026-01-11' },
  { id: 'e-3', code: '198805052012011003', name: 'Citra Dewi', email: 'citra.dewi@inventra.go.id', phone: '0814-5555-6666', position_id: null, department_id: null, office_id: 'o-bdg', status: 'inactive', created_at: '2026-01-12', updated_at: '2026-01-12' }
]

export const employeeStore = createStore<Employee>(employeeSeed)
