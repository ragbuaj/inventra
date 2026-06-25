import type { User, BadgeColor } from '~/types'
import { createStore } from './helpers'

/** Selectable roles (mockup `PERAN`). Mock-first; later sourced from identity.roles. */
export const ROLES = ['Superadmin', 'Kepala Kanwil', 'Kepala Unit', 'Asset Manager', 'Staf'] as const

/** Assigned-office options (mockup `KANTOR`). Later sourced from the offices API. */
export const KANTOR_OPTIONS = [
  'Kantor Pusat', 'Kanwil DKI Jakarta', 'Cabang Jakarta Selatan',
  'Cabang Jakarta Pusat', 'Outlet Blok M', 'Outlet Kemang'
] as const

/** Linked-employee options (mockup `PEGAWAI`). Later sourced from the employees API. */
export const PEGAWAI_OPTIONS = [
  'Andi Saputra', 'Rina Putri', 'Siti Aminah', 'Dewi Lestari', 'Budi Hartono',
  'Bambang Sukasno', 'Eko Prasetyo', 'Maya Sari', 'Fajar Nugroho', 'Agus Salim', 'Putri Maharani'
] as const

/** Role → semantic badge color (mockup `PERAN_C`). Unknown roles fall back to neutral. */
export const roleBadgeColor: Record<string, BadgeColor> = {
  'Superadmin': 'primary',
  'Kepala Kanwil': 'info',
  'Kepala Unit': 'info',
  'Asset Manager': 'warning',
  'Staf': 'neutral'
}

export function userRoleColor(peran: string): BadgeColor {
  return roleBadgeColor[peran] ?? 'neutral'
}

function u(
  nama: string, email: string, peran: string, kantor: string, pegawai: string,
  login: User['login'], status: User['status'], created_at: string
): User {
  return { id: email, nama, email, peran, kantor, pegawai, login, status, created_at }
}

export const userSeed: User[] = [
  u('Super Admin', 'admin@inventra.go.id', 'Superadmin', 'Kantor Pusat', '', 'email', 'active', '2026-01-02'),
  u('Bambang Sukasno', 'bambang.s@inventra.go.id', 'Kepala Kanwil', 'Kanwil DKI Jakarta', 'Bambang Sukasno', 'email', 'active', '2026-01-03'),
  u('Siti Aminah', 'siti.aminah@inventra.go.id', 'Kepala Unit', 'Cabang Jakarta Selatan', 'Siti Aminah', 'email', 'active', '2026-01-04'),
  u('Dewi Lestari', 'dewi.lestari@inventra.go.id', 'Asset Manager', 'Cabang Jakarta Selatan', 'Dewi Lestari', 'google', 'active', '2026-01-05'),
  u('Andi Saputra', 'andi.saputra@inventra.go.id', 'Asset Manager', 'Cabang Jakarta Selatan', 'Andi Saputra', 'email', 'active', '2026-01-06'),
  u('Rina Putri', 'rina.putri@inventra.go.id', 'Staf', 'Cabang Jakarta Selatan', 'Rina Putri', 'google', 'active', '2026-01-07'),
  u('Budi Hartono', 'budi.hartono@inventra.go.id', 'Staf', 'Cabang Jakarta Selatan', 'Budi Hartono', 'email', 'suspended', '2026-01-08'),
  u('Eko Prasetyo', 'eko.prasetyo@inventra.go.id', 'Staf', 'Outlet Blok M', 'Eko Prasetyo', 'google', 'active', '2026-01-09'),
  u('Maya Sari', 'maya.sari@inventra.go.id', 'Staf', 'Outlet Kemang', 'Maya Sari', 'email', 'inactive', '2026-01-10'),
  u('Fajar Nugroho', 'fajar.nugroho@inventra.go.id', 'Staf', 'Cabang Jakarta Pusat', 'Fajar Nugroho', 'google', 'active', '2026-01-11'),
  u('Agus Salim', 'agus.salim@inventra.go.id', 'Asset Manager', 'Cabang Jakarta Selatan', 'Agus Salim', 'email', 'active', '2026-01-12'),
  u('Putri Maharani', 'putri.maharani@inventra.go.id', 'Staf', 'Cabang Jakarta Selatan', 'Putri Maharani', 'email', 'active', '2026-01-13')
]

export const userStore = createStore<User>(userSeed)
