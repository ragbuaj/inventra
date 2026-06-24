import type { Floor, Room } from '~/types'
import { createStore } from './helpers'
import type { MockStore } from './helpers'

// Floors keyed by office_id
const floorSeeds: Floor[] = [
  { id: 'fl-pusat-1', office_id: 'o-pusat', nama: 'Lantai 1', lantai: 1, created_at: '2026-01-10' },
  { id: 'fl-pusat-2', office_id: 'o-pusat', nama: 'Lantai 2', lantai: 2, created_at: '2026-01-10' },
  { id: 'fl-pusat-3', office_id: 'o-pusat', nama: 'Lantai 3', lantai: 3, created_at: '2026-01-10' },
  { id: 'fl-jkt-1', office_id: 'o-jkt', nama: 'Lantai 1', lantai: 1, created_at: '2026-01-11' },
  { id: 'fl-jkt-2', office_id: 'o-jkt', nama: 'Lantai 2', lantai: 2, created_at: '2026-01-11' }
]

// Rooms keyed by floor_id
const roomSeeds: Room[] = [
  { id: 'rm-pusat-1-a', floor_id: 'fl-pusat-1', office_id: 'o-pusat', nama: 'Ruang Lobby', kode: 'PST-L1-A', created_at: '2026-01-10' },
  { id: 'rm-pusat-1-b', floor_id: 'fl-pusat-1', office_id: 'o-pusat', nama: 'Ruang Rapat A', kode: 'PST-L1-B', created_at: '2026-01-10' },
  { id: 'rm-pusat-2-a', floor_id: 'fl-pusat-2', office_id: 'o-pusat', nama: 'Ruang Operasional', kode: 'PST-L2-A', created_at: '2026-01-10' },
  { id: 'rm-pusat-2-b', floor_id: 'fl-pusat-2', office_id: 'o-pusat', nama: 'Ruang Direktur', kode: 'PST-L2-B', created_at: '2026-01-10' },
  { id: 'rm-pusat-3-a', floor_id: 'fl-pusat-3', office_id: 'o-pusat', nama: 'Aula Utama', kode: 'PST-L3-A', created_at: '2026-01-10' },
  { id: 'rm-jkt-1-a', floor_id: 'fl-jkt-1', office_id: 'o-jkt', nama: 'Ruang Pelayanan', kode: 'JKT-L1-A', created_at: '2026-01-11' },
  { id: 'rm-jkt-2-a', floor_id: 'fl-jkt-2', office_id: 'o-jkt', nama: 'Ruang Rapat', kode: 'JKT-L2-A', created_at: '2026-01-11' }
]

export const floorStore: MockStore<Floor> = createStore<Floor>(floorSeeds)
export const roomStore: MockStore<Room> = createStore<Room>(roomSeeds)
