const idrFormatter = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 })

/** Formats a backend decimal string / number as IDR; '—' when absent or invalid. */
export function formatRupiah(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  return idrFormatter.format(n)
}

export function formatDate(iso: string | null, opts: { withTime?: boolean } = {}): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    ...(opts.withTime ? { timeStyle: 'short' } : {})
  }).format(d)
}
