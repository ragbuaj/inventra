import type { Employee } from '~/types'
import { createStore } from './helpers'

export const employeeSeed: Employee[] = [
  { id: 'e-1', nip: '199001012015011001', nama: 'Andi Pratama', email: 'andi.pratama@inventra.go.id', telepon: '0812-1111-2222', jabatan: 'Kepala Kantor', departemen: 'Umum', office_id: 'o-jkt', status: 'active', created_at: '2026-01-10' },
  { id: 'e-2', nip: '199203122016012002', nama: 'Bunga Lestari', email: 'bunga.lestari@inventra.go.id', telepon: '0813-3333-4444', jabatan: 'Staf', departemen: 'Keuangan', office_id: 'o-jkt', status: 'active', created_at: '2026-01-11' },
  { id: 'e-3', nip: '198805052012011003', nama: 'Citra Dewi', email: 'citra.dewi@inventra.go.id', telepon: '0814-5555-6666', jabatan: 'Kepala Unit', departemen: 'Aset', office_id: 'o-bdg', status: 'inactive', created_at: '2026-01-12' }
]

export const employeeStore = createStore<Employee>(employeeSeed)
