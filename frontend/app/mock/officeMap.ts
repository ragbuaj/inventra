import type { MapOffice, OfficeJenis } from '~/types'

/**
 * Jenis → i18n label key, semantic color token, and pin/legend Tailwind classes.
 * Pin colors map the mockup's pins to semantic tokens:
 *   Pusat→primary, Wilayah→info, Cabang→warning, Outlet→neutral (via pinVar CSS vars).
 */
export const jenisMeta: Record<OfficeJenis, {
  labelKey: string
  pinVar: string
  softBg: string
  softText: string
  icon: string
}> = {
  Pusat: { labelKey: 'map.jenis.pusat', pinVar: '--pin-pusat', softBg: 'bg-primary/10', softText: 'text-primary', icon: 'i-lucide-landmark' },
  Wilayah: { labelKey: 'map.jenis.wilayah', pinVar: '--pin-wilayah', softBg: 'bg-info/10', softText: 'text-info', icon: 'i-lucide-building-2' },
  Cabang: { labelKey: 'map.jenis.cabang', pinVar: '--pin-cabang', softBg: 'bg-warning/10', softText: 'text-warning', icon: 'i-lucide-building' },
  Outlet: { labelKey: 'map.jenis.outlet', pinVar: '--pin-outlet', softBg: 'bg-neutral/10', softText: 'text-dimmed', icon: 'i-lucide-store' }
}

export const JENIS_ORDER: OfficeJenis[] = ['Pusat', 'Wilayah', 'Cabang', 'Outlet']

export const mapOffices: MapOffice[] = [
  { id: 'o1', nama: 'Kantor Pusat', kode: 'PST', jenis: 'Pusat', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. Medan Merdeka Barat No. 1, Jakarta Pusat', aset: 94, lat: -6.1754, lng: 106.8272 },
  { id: 'o2', nama: 'Kanwil DKI Jakarta', kode: 'KW-DKI', jenis: 'Wilayah', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. Jend. Sudirman Kav. 5, Jakarta Pusat', aset: 56, lat: -6.2088, lng: 106.8200 },
  { id: 'o3', nama: 'Cabang Jakarta Selatan', kode: 'JKT01', jenis: 'Cabang', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Jl. TB Simatupang No. 22, Jakarta Selatan', aset: 96, lat: -6.2920, lng: 106.8000 },
  { id: 'o4', nama: 'Cabang Jakarta Pusat', kode: 'JKT02', jenis: 'Cabang', kota: 'Jakarta Pusat', prov: 'DKI Jakarta', alamat: 'Jl. M.H. Thamrin No. 10, Jakarta Pusat', aset: 112, lat: -6.1944, lng: 106.8229 },
  { id: 'o5', nama: 'Outlet Blok M', kode: 'JKT01-BM', jenis: 'Outlet', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Blok M Square Lt. 2, Jakarta Selatan', aset: 28, lat: -6.2443, lng: 106.7992 },
  { id: 'o6', nama: 'Outlet Kemang', kode: 'JKT01-KM', jenis: 'Outlet', kota: 'Jakarta Selatan', prov: 'DKI Jakarta', alamat: 'Jl. Kemang Raya No. 8, Jakarta Selatan', aset: 19, lat: -6.2601, lng: 106.8140 },
  { id: 'o7', nama: 'Cabang Bekasi', kode: 'BKS01', jenis: 'Cabang', kota: 'Bekasi', prov: 'Jawa Barat', alamat: 'Jl. Ahmad Yani No. 1, Bekasi', aset: 64, lat: -6.2383, lng: 106.9756 },
  { id: 'o8', nama: 'Cabang Tangerang', kode: 'TGR01', jenis: 'Cabang', kota: 'Tangerang', prov: 'Banten', alamat: 'Jl. Jend. Sudirman No. 3, Tangerang', aset: 48, lat: -6.1783, lng: 106.6319 },
  { id: 'o9', nama: 'Outlet Depok', kode: 'DPK01', jenis: 'Outlet', kota: 'Depok', prov: 'Jawa Barat', alamat: 'Jl. Margonda Raya No. 100, Depok', aset: 22, lat: -6.3833, lng: 106.8167 }
]
