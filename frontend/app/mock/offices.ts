import { createStore } from './helpers'

/**
 * Standalone mock office shape for the mock global-search aggregator
 * (`useGlobalSearch`). Deliberately independent of the real `Office` type
 * (which is the English backend contract) — delete this file once global
 * search is wired to the real `/search` endpoint.
 */
export interface MockOffice {
  id: string
  nama: string
  kode: string
  tipe: 'pusat' | 'kanwil' | 'cabang' | 'unit'
  parent_id: string | null
  provinsi: string
  kota: string
  alamat: string
  active: boolean
  created_at: string
}

export const officeSeed: MockOffice[] = [
  { id: 'o-pusat', nama: 'Kantor Pusat', kode: 'PST', tipe: 'pusat', parent_id: null, provinsi: 'DKI Jakarta', kota: 'Jakarta Pusat', alamat: 'Jl. Merdeka No. 1', active: true, created_at: '2026-01-02' },
  { id: 'o-jkt', nama: 'Kanwil Jakarta', kode: 'JKT01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Sudirman No. 10', active: true, created_at: '2026-01-03' },
  { id: 'o-jkt-a', nama: 'Cabang Kebayoran', kode: 'JKT01-A', tipe: 'cabang', parent_id: 'o-jkt', provinsi: 'DKI Jakarta', kota: 'Jakarta Selatan', alamat: 'Jl. Kebayoran No. 5', active: true, created_at: '2026-01-04' },
  { id: 'o-bdg', nama: 'Kanwil Bandung', kode: 'BDG01', tipe: 'kanwil', parent_id: 'o-pusat', provinsi: 'Jawa Barat', kota: 'Bandung', alamat: 'Jl. Asia Afrika No. 8', active: true, created_at: '2026-01-05' },
  { id: 'o-bdg-a', nama: 'Cabang Cimahi', kode: 'BDG01-A', tipe: 'cabang', parent_id: 'o-bdg', provinsi: 'Jawa Barat', kota: 'Cimahi', alamat: 'Jl. Cimahi Raya No. 3', active: false, created_at: '2026-01-06' }
]

export const officeStore = createStore<MockOffice>(officeSeed)
